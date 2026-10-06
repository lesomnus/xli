package cfg

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
)

// Loader loads the configuration of type T for one application.
type Loader[T any] struct {
	name   string
	prefix string
	root   *T
	schema *schema
	opts   options

	// base is what root held when the loader was made: the defaults.
	base T

	// file is the fixed path of a loader made by NewFile.
	file string

	cur atomic.Pointer[Snapshot[T]]

	mu sync.Mutex
	// first is the snapshot of the first load, whose Config is root.
	first *Snapshot[T]
	// in is what the first load read, for Watch.
	in *inputs

	// skip are commands the Load handler does not load the configuration
	// for, and tolerant those it runs even when the configuration fails to
	// load: `config env` and `config` of NewCmdConfig.
	skip, tolerant []*xli.Command
	// failed is the load that failed for a tolerant command.
	failed atomic.Pointer[failure[T]]
}

// failure is a load that failed, and the snapshot of what it read.
type failure[T any] struct {
	s   *Snapshot[T]
	err error
}

// Snapshot is one load of the configuration.
type Snapshot[T any] struct {
	// Config is the loaded configuration.
	Config *T
	// Path is the file that was read; empty when there was none.
	Path string
	// Revision identifies the file's content (the start of its SHA-256); empty
	// when there was no file.
	Revision string
	// Unknown are the environment variables under the prefix that no field
	// reads and nothing claims, which is what a typo looks like.
	Unknown []string
	// Warnings are problems that do not stop the load: a secret file that
	// cannot be read yet, which is read again when the secret is used.
	Warnings []error

	schema  *schema
	origins map[string]Origin
	// written are the values of the file read through references, as
	// written, for Print.
	written map[string]any
}

type options struct {
	paths    []string
	claims   []string
	schemes  map[string]Resolver
	environ  func() []string
	interval time.Duration
	links    bool
}

// Option configures a Loader.
type Option func(*options)

// WithPaths sets the files tried, in order, when no file is named. The default
// is "<name>.yaml" and "<name>.yml"; WithPaths() tries none.
func WithPaths(paths ...string) Option {
	return func(o *options) { o.paths = append([]string{}, paths...) }
}

// Claims declares environment variables under the prefix that the application
// reads itself, by what follows the prefix: Claims("LDAP_KEY_") for an
// application "roster" claims every ROSTER_LDAP_KEY_*. They are not reported as
// unknown.
func Claims(prefixes ...string) Option {
	return func(o *options) { o.claims = append(o.claims, prefixes...) }
}

// WithScheme registers a reference scheme: `${name:...}` is resolved by f.
func WithScheme(name string, f Resolver) Option {
	return func(o *options) {
		if o.schemes == nil {
			o.schemes = map[string]Resolver{}
		}
		o.schemes[name] = f
	}
}

// WithEnviron sets where the environment is read from; the default is
// os.Environ. It is for tests.
func WithEnviron(f func() []string) Option {
	return func(o *options) { o.environ = f }
}

// WithInterval sets how often Watch checks the file; the default is 5 seconds.
func WithInterval(d time.Duration) Option {
	return func(o *options) { o.interval = d }
}

// KeepServiceLinks reports the environment variables Kubernetes sets for every
// service in a namespace (`<SERVICE>_PORT_*`, `<SERVICE>_SERVICE_HOST`) as
// unknown like any other. By default they are left out, since a namespace
// whose services are named after the application gives it dozens of them.
func KeepServiceLinks() Option {
	return func(o *options) { o.links = true }
}

// New is a loader for the application named name, loading into root.
//
// The name gives the files tried when none is named ("<name>.yaml",
// "<name>.yml") and the prefix of every environment variable: "go-app" reads
// GO_APP_*.
//
// What root holds now is the defaults, the lowest layer: every load starts
// from a copy of it.
//
//	c := Config{Db: DbConfig{MaxConn: 10}}
//	l := cfg.New("roster", &c)
//
// The first load is read into root, and root is what Bind and Origin take
// pointers into.
//
// It panics if T is not a struct, if two fields end up with the same name or
// environment variable, or if name is empty: those are programs built wrong.
func New[T any](name string, root *T, opts ...Option) *Loader[T] {
	if name == "" {
		panic("cfg: an application has to be named: its file and its environment variables are named after it")
	}
	if root == nil {
		panic("cfg: root is nil")
	}
	l := newLoader[T](envPrefix(name), opts)
	l.name = name
	l.root = root
	l.base = clone(reflect.ValueOf(root).Elem()).Interface().(T)
	if l.opts.paths == nil {
		l.opts.paths = []string{name + ".yaml", name + ".yml"}
	}
	return l
}

// NewFile is a loader for a file of its own, such as a policy file beside the
// configuration: the file at path, which must exist, and no environment
// variables or flags.
func NewFile[T any](path string, opts ...Option) *Loader[T] {
	if path == "" {
		panic("cfg: NewFile needs a path")
	}
	l := newLoader[T]("", opts)
	l.file = path
	l.root = new(T)
	return l
}

func newLoader[T any](prefix string, opts []Option) *Loader[T] {
	l := &Loader[T]{prefix: prefix}
	for _, opt := range opts {
		opt(&l.opts)
	}
	if l.opts.environ == nil {
		l.opts.environ = os.Environ
	}
	if l.opts.interval <= 0 {
		l.opts.interval = 5 * time.Second
	}

	s, err := newSchema(reflect.TypeFor[T](), prefix)
	if err != nil {
		panic(err)
	}
	l.schema = s
	return l
}

// Name is the application's name.
func (l *Loader[T]) Name() string { return l.name }

// Prefix is what every environment variable the loader reads starts with,
// without the trailing "_".
func (l *Loader[T]) Prefix() string { return l.prefix }

// Paths are the files tried, in order, when no file is named.
func (l *Loader[T]) Paths() []string { return slices.Clone(l.opts.paths) }

// EnvNames are the environment variables the configuration reads, in the order
// of its fields.
func (l *Loader[T]) EnvNames() []string {
	vs := []string{}
	for _, f := range l.schema.fields {
		if f.env != "" {
			vs = append(vs, f.env)
		}
	}
	return vs
}

// Current is the snapshot in force: the latest one loaded, or nil before the
// first load.
func (l *Loader[T]) Current() *Snapshot[T] {
	return l.cur.Load()
}

// Origin is where the value of the field ptr points to in the root came from.
// ok is false if ptr is not a leaf field of the root, or before the first load.
func (l *Loader[T]) Origin(ptr any) (Origin, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.first == nil {
		return Origin{}, false
	}
	return l.first.Origin(ptr)
}

// Origin is where the value of the field ptr points to in s.Config came from.
// ok is false if ptr is not a leaf field of s.Config.
func (s *Snapshot[T]) Origin(ptr any) (Origin, bool) {
	return s.origin(reflect.ValueOf(s.Config).Elem(), ptr)
}

func (s *Snapshot[T]) origin(root reflect.Value, ptr any) (Origin, bool) {
	f, ok := s.schema.find(root, ptr)
	if !ok {
		return Origin{}, false
	}
	return s.originOf(f), true
}

func (s *Snapshot[T]) originOf(f *field) Origin {
	if o, ok := s.origins[f.key]; ok {
		return o
	}
	return Origin{Source: Unset, Key: f.key}
}

// Origins are the origins of every field, in the order of the fields.
func (s *Snapshot[T]) Origins() []Origin {
	vs := make([]Origin, len(s.schema.fields))
	for i, f := range s.schema.fields {
		vs[i] = s.originOf(f)
	}
	return vs
}

// inputs are what a load reads besides the file's content.
type inputs struct {
	path    string // named by the user; "" to try the default paths
	noFile  bool   // read no file at all (--config=)
	environ []string
	bound   []*binding
}

// Load reads the configuration from the file at path (or the first of the
// default paths that exists, when path is empty), the environment environ (in
// the form of os.Environ) and the bound flags among flags, into the root: the
// snapshot's Config is the root. It is what the Load handler calls.
func (l *Loader[T]) Load(path string, environ []string, flags ...flg.Flag) (*Snapshot[T], error) {
	bound, err := l.bindings(flags)
	if err != nil {
		return nil, err
	}
	s, err := l.load(&inputs{path: path, environ: environ, bound: bound})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// load is Load with its inputs. A load that fails returns what it read before
// it failed, if anything, for config to print.
func (l *Loader[T]) load(in *inputs) (*Snapshot[T], error) {
	s, err := l.read(in)
	if err != nil {
		return s, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	*l.root = *s.Config
	s.Config = l.root
	l.first, l.in = s, in
	l.cur.Store(s)
	return s, nil
}

// Read is Load without touching the root or the loader's state: the snapshot
// has a Config of its own.
func (l *Loader[T]) Read(path string, environ []string, flags ...flg.Flag) (*Snapshot[T], error) {
	bound, err := l.bindings(flags)
	if err != nil {
		return nil, err
	}
	s, err := l.read(&inputs{path: path, environ: environ, bound: bound})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// bindings are the flags among fs bound to this loader.
func (l *Loader[T]) bindings(fs []flg.Flag) ([]*binding, error) {
	vs := []*binding{}
	for _, f := range fs {
		b := bindingOf(f)
		if b == nil || b.owner != any(l) {
			continue
		}
		for _, c := range vs {
			if c.field.covers(b.field) || b.field.covers(c.field) {
				key := b.field.key
				if len(c.field.key) > len(key) {
					key = c.field.key
				}
				return nil, fmt.Errorf("cfg: --%s and --%s are both bound to %s", c.name, b.name, key)
			}
		}
		vs = append(vs, b)
	}
	return vs, nil
}

// file is the content of the configuration file to read, and its path; no
// path when there is none.
func (l *Loader[T]) readFile(in *inputs) (path string, content []byte, err error) {
	if l.file != "" {
		b, err := os.ReadFile(l.file)
		return l.file, b, err
	}
	if in.noFile {
		return "", nil, nil
	}
	if in.path != "" {
		b, err := os.ReadFile(in.path)
		return in.path, b, err
	}
	for _, p := range l.opts.paths {
		b, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		return p, b, err
	}
	return "", nil, nil
}

func (l *Loader[T]) read(in *inputs) (*Snapshot[T], error) {
	path, content, err := l.readFile(in)
	if err != nil {
		return nil, err
	}
	return l.build(in, path, content)
}

// build loads the configuration: defaults, then the file, then the
// environment, then the flags; then validates it. On an error, the snapshot is
// what it read.
func (l *Loader[T]) build(in *inputs, path string, content []byte) (*Snapshot[T], error) {
	cfg := new(T)
	rv := reflect.ValueOf(cfg).Elem()
	s := &Snapshot[T]{
		Config:  cfg,
		Path:    path,
		schema:  l.schema,
		origins: map[string]Origin{},
		written: map[string]any{},
	}

	var es errs
	r := &resolver{lookup: lookupIn(in.environ), schemes: l.opts.schemes}
	// Values from the environment and flags are taken as they are.
	given := r.asGiven()

	// The defaults: a copy of what the root held when the loader was made, so
	// that reading into the configuration never writes into them.
	rv.Set(clone(reflect.ValueOf(&l.base).Elem()))
	for _, f := range l.schema.fields {
		if v, ok := f.value(rv, false); ok && !v.IsZero() {
			s.origins[f.key] = Origin{Source: Default, Key: f.key}
		}
	}
	for _, b := range in.bound {
		o := Origin{Source: Default, Key: b.field.key}
		v := reflect.New(b.field.typ).Elem()
		given.refs = nil
		ok, err := b.applyDefault(v, given)
		if err != nil {
			es.add(o, fmt.Errorf("default of --%s: %w", b.name, err))
			if !isPending(err) {
				continue
			}
		}
		if !ok {
			continue
		}
		o.Refs = refsOf(v, given)
		l.set(rv, b.field, v, false)
		l.mark(s, b.field, o)
	}

	// The file.
	if path != "" {
		s.Revision = revision(path, content)
		es = append(es, l.decodeFile(s, rv, r, path, content)...)
	}

	// The environment.
	if l.prefix != "" {
		es = append(es, l.applyEnv(s, rv, given, in.environ)...)
	}

	// The flags.
	for _, b := range in.bound {
		if b.flag.Count() == 0 {
			continue
		}
		o := Origin{Source: Flag, Key: b.field.key, Name: "--" + b.name}
		v := reflect.New(b.field.typ).Elem()
		given.refs = nil
		cleared, err := b.apply(v, given)
		if err != nil {
			es.add(o, err)
			if !isPending(err) {
				continue
			}
		}
		o.Cleared = cleared
		o.Refs = refsOf(v, given)
		l.set(rv, b.field, v, cleared)
		l.mark(s, b.field, o)
	}

	es = append(es, validate(rv, s)...)

	var fatal errs
	for _, err := range es {
		if isPending(err) {
			s.Warnings = append(s.Warnings, err)
			continue
		}
		fatal = append(fatal, err)
	}
	if err := fatal.join(); err != nil {
		return s, err
	}
	return s, nil
}

// set sets the field f of root to v. A field cleared is set to its zero value
// without making the blocks on the way to it: a nil block stays nil until a
// value is read into it.
func (l *Loader[T]) set(root reflect.Value, f *field, v reflect.Value, cleared bool) {
	if cleared {
		if fv, ok := f.value(root, false); ok {
			fv.SetZero()
		}
		return
	}
	fv, _ := f.value(root, true)
	fv.Set(v)
}

// mark records o as the origin of f, or of every leaf in f if it is a group.
func (l *Loader[T]) mark(s *Snapshot[T], f *field, o Origin) {
	for _, g := range l.schema.fields {
		if f.covers(g) {
			o.Key = g.key
			s.origins[g.key] = o
		}
	}
}

func (l *Loader[T]) decodeFile(s *Snapshot[T], rv reflect.Value, r *resolver, path string, content []byte) (es errs) {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF}) // a UTF-8 BOM
	if documents(content) > 1 {
		es.add(Origin{Source: File, Name: path}, errors.New("more than one document"))
		return es
	}
	f, err := parser.ParseBytes(content, 0)
	if err != nil {
		if unquotedRef(content, err) {
			// "{" and "}" end a flow mapping, so an unquoted reference in one
			// is a syntax error rather than a value.
			err = fmt.Errorf("%w\n(a reference inside [...] or {...} must be quoted, e.g. [\"${env:NAME}\"])", err)
		}
		es.add(Origin{Source: File, Name: path}, err)
		return es
	}
	d := &decoder{r: r, file: path}
	body, err := d.body(f)
	if err != nil {
		es.add(Origin{Source: File, Name: path}, err)
		return es
	}
	if body == nil {
		return nil
	}
	d.leaf = func(key string, n ast.Node, refs []string, cleared bool) {
		o := d.at(n, key)
		o.Refs = slices.Clone(refs)
		o.Cleared = cleared
		if _, ok := l.schema.byKey[key]; ok {
			s.origins[key] = o
			if len(refs) > 0 {
				s.written[key] = d.asWritten(n)
			}
			return
		}
		// A block given as null: every leaf in it is cleared.
		for _, f := range l.schema.fields {
			if key == "" || strings.HasPrefix(f.key, key+".") {
				o.Key = f.key
				s.origins[f.key] = o
			}
		}
	}
	return d.decode(body, rv, "", true)
}

// documents counts the document markers (`---`) of content. The parser keeps
// one document of `---` twice and drops what follows, so they are counted
// before parsing.
func documents(content []byte) int {
	n := 0
	for _, t := range lexer.Tokenize(string(content)) {
		if t.Type == token.DocumentHeaderType {
			n++
		}
	}
	return n
}

// unquotedRef reports whether the line a syntax error is on holds a reference
// that is not quoted.
func unquotedRef(content []byte, err error) bool {
	var se *yaml.SyntaxError
	if !errors.As(err, &se) || se.Token == nil || se.Token.Position == nil {
		return false
	}
	lines := strings.Split(string(content), "\n")
	i := se.Token.Position.Line - 1
	if i < 0 || i >= len(lines) {
		return false
	}
	line := lines[i]
	for at := strings.Index(line, "${"); at >= 0; {
		if at == 0 || (line[at-1] != '"' && line[at-1] != '\'') {
			return true
		}
		next := strings.Index(line[at+2:], "${")
		if next < 0 {
			break
		}
		at += 2 + next
	}
	return false
}

func (l *Loader[T]) applyEnv(s *Snapshot[T], rv reflect.Value, r *resolver, environ []string) (es errs) {
	lookup := r.lookup
	for _, f := range l.schema.fields {
		if f.env == "" {
			continue
		}
		val, ok := lookup(f.env)
		if !ok {
			continue
		}

		o := Origin{Source: Env, Key: f.key, Name: f.env}
		if val == "" {
			o.Cleared = true
			l.set(rv, f, reflect.Value{}, true)
			s.origins[f.key] = o
			continue
		}
		v, _ := f.value(rv, true)
		r.refs = nil
		if err := setText(v, val, r); err != nil {
			es.add(o, err)
			if !isPending(err) {
				continue
			}
		}
		o.Refs = refsOf(v, r)
		s.origins[f.key] = o
	}

	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(name, l.prefix+"_") {
			continue
		}
		if _, ok := l.schema.byEnv[name]; ok {
			continue
		}
		if l.claimed(name) {
			continue
		}
		s.Unknown = append(s.Unknown, name)
	}
	slices.Sort(s.Unknown)
	s.Unknown = slices.Compact(s.Unknown)
	return es
}

// serviceLink matches the variables Kubernetes sets for every service in a
// namespace, named after the service.
var serviceLink = regexp.MustCompile(`_(SERVICE_HOST|SERVICE_PORT(_[A-Z0-9_]+)?|PORT|PORT_[0-9]+_(TCP|UDP)(_(ADDR|PORT|PROTO))?)$`)

func (l *Loader[T]) claimed(name string) bool {
	for _, p := range l.opts.claims {
		if strings.HasPrefix(name, l.prefix+"_"+p) {
			return true
		}
	}
	return !l.opts.links && serviceLink.MatchString(name)
}

// refsOf are the references the value v was read through.
func refsOf(v reflect.Value, r *resolver) []string {
	if v.CanAddr() {
		if sf, ok := v.Addr().Interface().(secretField); ok {
			return sf.refs()
		}
	}
	return slices.Clone(r.refs)
}

func lookupIn(environ []string) func(string) (string, bool) {
	m := make(map[string]string, len(environ))
	for _, kv := range environ {
		k, v, ok := strings.Cut(kv, "=")
		if ok {
			m[k] = v
		}
	}
	return func(name string) (string, bool) {
		v, ok := m[name]
		return v, ok
	}
}

// revision identifies a file's content: the start of its SHA-256, or "" when
// there is no file.
func revision(path string, content []byte) string {
	if path == "" {
		return ""
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:6])
}
