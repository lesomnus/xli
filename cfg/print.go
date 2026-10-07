package cfg

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// Print writes the configuration as YAML that reads back as the same
// configuration, each value commented with where it came from. Fields nothing
// set and that hold their zero value are left out.
//
// What is secret is redacted, so that the output can be pasted into a ticket:
//
//   - a Secret prints as its reference, or as "<redacted>" for a literal;
//   - so does a field tagged `cfg:",secret"`, at any depth;
//   - and, as payday did, every value under a name that says it is secret:
//     token, password, secret(s), seal, key(s), credential(s), and names
//     ending in _token, _password, _secret, _key or _keys; a tag is what
//     somebody forgets on the one field that matters;
//   - the password of a value named dsn, or with a name ending in _dsn
//     (dev_dsn), in a URL or as password=.
//
// A value read through references, such as `postgres://u:${env:PW}@h`, prints
// as it was written: it says where the secret is, not what it is.
//
// A block that decodes itself prints as it marshals, but for the embedded
// structs that hold nothing, such as an "Unimplemented..." type embedded for
// its methods: goccy/go-yaml writes one as a key of its own, which says
// nothing.
func (s *Snapshot[T]) Print(w io.Writer) error {
	p := &printer{
		schema:  s.schema,
		root:    reflect.ValueOf(s.Config).Elem(),
		origin:  s.originOf,
		written: s.written,
	}
	top, err := p.tree()
	if err != nil {
		return err
	}
	b := &strings.Builder{}
	top.render(b, 0)
	_, err = io.WriteString(w, b.String())
	return err
}

type printer struct {
	schema  *schema
	root    reflect.Value
	origin  func(*field) Origin
	written map[string]any
}

// pnode is a line of the output: a value, or a block of them.
type pnode struct {
	name     string
	value    string // in flow syntax; "" for a block
	comment  string
	children []*pnode
	// empty is what a block with nothing in it prints as: "{}" for one that
	// is there behind a pointer; "" leaves it out.
	empty string
}

func (n *pnode) render(b *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, c := range n.children {
		switch {
		case c.value != "":
			b.WriteString(indent + key(c.name) + ": " + c.value)
			if c.comment != "" {
				b.WriteString("  # " + c.comment)
			}
			b.WriteByte('\n')
		case c.holds():
			b.WriteString(indent + key(c.name) + ":\n")
			c.render(b, depth+1)
		case c.empty != "":
			b.WriteString(indent + key(c.name) + ": " + c.empty + "\n")
		}
	}
}

// holds reports whether a block has a line to print under it. A block whose
// blocks hold nothing printed as "client:" alone, which reads back as null and
// clears it.
func (n *pnode) holds() bool {
	for _, c := range n.children {
		if c.value != "" || c.empty != "" || c.holds() {
			return true
		}
	}
	return false
}

func (p *printer) tree() (*pnode, error) {
	top := &pnode{}
	blocks := map[string]*pnode{"": top}
	for _, f := range p.schema.all {
		parent, ok := blocks[parentKey(f.key)]
		if !ok {
			// In a block that is not there.
			continue
		}
		name := f.key[strings.LastIndexByte(f.key, '.')+1:]

		v, ok := f.value(p.root, false)
		if !ok {
			continue
		}
		if f.group {
			if v.Kind() == reflect.Pointer && v.IsNil() {
				// Not there: say so if something said so.
				if o, ok := p.clearedIn(f); ok {
					parent.children = append(parent.children, &pnode{name: name, value: "null", comment: o.comment()})
				}
				continue
			}
			n := &pnode{name: name}
			if v.Kind() == reflect.Pointer {
				n.empty = "{}"
			}
			blocks[f.key] = n
			parent.children = append(parent.children, n)
			continue
		}

		o := p.origin(f)
		if o.Source == SourceUnset && v.IsZero() {
			continue
		}
		val, err := p.leaf(f, v, o)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.key, err)
		}
		parent.children = append(parent.children, &pnode{name: name, value: val, comment: o.comment()})
	}
	return top, nil
}

// clearedIn is the origin of a value in the block g, which is not there.
func (p *printer) clearedIn(g *field) (Origin, bool) {
	for _, f := range p.schema.fields {
		if g.covers(f) {
			if o := p.origin(f); o.Source != SourceUnset {
				return o, true
			}
		}
	}
	return Origin{}, false
}

func parentKey(key string) string {
	if i := strings.LastIndexByte(key, '.'); i >= 0 {
		return key[:i]
	}
	return ""
}

func (p *printer) leaf(f *field, v reflect.Value, o Origin) (string, error) {
	name := f.key[strings.LastIndexByte(f.key, '.')+1:]
	secret := f.secret
	for _, n := range strings.Split(f.key, ".") {
		// Under a block named keys, as much as a field named key.
		secret = secret || secretName(n)
	}
	if w, ok := p.written[f.key]; ok && o.Source == SourceFile && !holdsSecret(f.typ) {
		// Read through references from the file, and not set over since:
		// as written.
		b := &strings.Builder{}
		writeWritten(b, w, secret, name, false)
		return b.String(), nil
	}
	b := &strings.Builder{}
	if err := writeValue(b, v, secret, name, false); err != nil {
		return "", err
	}
	return b.String(), nil
}

// writeWritten writes a value as it was written in the file (anyOf of it, its
// references not resolved). Strings are written as they are, in the file's
// own syntax; under a secret name, only a reference is.
func writeWritten(b *strings.Builder, v any, secret bool, name string, flow bool) {
	switch v := v.(type) {
	case string:
		switch {
		case secret && !isRef(v):
			b.WriteString(redacted)
		case dsnName(name):
			b.WriteString(scalar(redactDsn(v), flow))
		default:
			b.WriteString(scalar(v, flow))
		}
	case rawScalar:
		if secret {
			b.WriteString(redacted)
			return
		}
		b.WriteString(string(v))
	case map[string]any:
		names := slices.Sorted(func(yield func(string) bool) {
			for k := range v {
				if !yield(k) {
					return
				}
			}
		})
		b.WriteByte('{')
		for i, k := range names {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(mapKey(k) + ": ")
			writeWritten(b, v[k], secret || secretName(k), k, true)
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeWritten(b, e, secret, name, true)
		}
		b.WriteByte(']')
	default:
		if secret && v != nil {
			b.WriteString(redacted)
			return
		}
		writeFlow(b, v)
	}
}

// writeValue writes v in flow syntax. A string is written so that the file
// reads it back as it is: its "$" doubled.
func writeValue(b *strings.Builder, v reflect.Value, secret bool, name string, flow bool) error {
	if !v.IsValid() {
		b.WriteString("null")
		return nil
	}
	t := v.Type()
	if isSecret(t) {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				b.WriteString("null")
				return nil
			}
			v = v.Elem()
		}
		// The reference as written, "<redacted>", or "" if not given.
		s := v.Interface().(fmt.Stringer).String()
		if s == redacted {
			b.WriteString(redacted)
		} else {
			b.WriteString(scalar(s, flow))
		}
		return nil
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			b.WriteString("null")
			return nil
		}
		return writeValue(b, v.Elem(), secret, name, flow)
	}

	if secret && v.Kind() != reflect.Struct && v.Kind() != reflect.Slice && v.Kind() != reflect.Array && v.Kind() != reflect.Map {
		if v.IsZero() && v.Kind() == reflect.String {
			b.WriteString(`""`)
		} else {
			b.WriteString(redacted)
		}
		return nil
	}
	if t == durationType {
		b.WriteString(time.Duration(v.Int()).String())
		return nil
	}
	if (v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.IsNil() {
		b.WriteString("null")
		return nil
	}
	if t == bytesType {
		if secret {
			b.WriteString(redacted)
			return nil
		}
		b.WriteString(`!!binary "` + base64.StdEncoding.EncodeToString(v.Bytes()) + `"`)
		return nil
	}
	if tm, ok := addressable(v).Interface().(encoding.TextMarshaler); ok {
		text, err := tm.MarshalText()
		if err != nil {
			return err
		}
		if secret {
			b.WriteString(redacted)
			return nil
		}
		b.WriteString(scalar(escapeRefs(string(text)), flow))
		return nil
	}
	if readsItself(t) {
		// A YAML marshaler: printed as it marshals, with the names that say
		// what is secret redacted.
		out, err := yaml.Marshal(addressable(v).Interface())
		if err != nil {
			return err
		}
		var a any
		if err := yaml.UnmarshalWithOptions(out, &a, yaml.UseOrderedMap()); err != nil {
			return err
		}
		names := map[string]bool{}
		markers(addressable(v), names, map[uintptr]bool{})
		if len(names) > 0 {
			a = dropMarkers(a, names)
		}
		writePlain(b, a, secret, name, flow)
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if dsnName(name) {
			s = redactDsn(s)
		}
		b.WriteString(scalar(escapeRefs(s), flow))
	case reflect.Bool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		b.WriteString(strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		b.WriteString(formatFloat(v.Float()))
	case reflect.Slice, reflect.Array:
		b.WriteByte('[')
		for i := range v.Len() {
			if i > 0 {
				b.WriteString(", ")
			}
			if err := writeValue(b, v.Index(i), secret, name, true); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case reflect.Map:
		type item struct {
			key string
			v   reflect.Value
		}
		items := []item{}
		for it := v.MapRange(); it.Next(); {
			items = append(items, item{textOf(it.Key()), it.Value()})
		}
		slices.SortFunc(items, func(a, b item) int { return strings.Compare(a.key, b.key) })
		b.WriteByte('{')
		for i, it := range items {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(mapKey(it.key) + ": ")
			if err := writeValue(b, it.v, secret || secretName(it.key), it.key, true); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case reflect.Struct:
		b.WriteByte('{')
		n := 0
		for _, f := range fieldsOf(t) {
			fv, ok := fieldAt(v, f.index)
			if !ok || fv.IsZero() {
				continue
			}
			if n++; n > 1 {
				b.WriteString(", ")
			}
			b.WriteString(key(f.name) + ": ")
			if err := writeValue(b, fv, secret || f.secret || secretName(f.name), f.name, true); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		return fmt.Errorf("cannot print %s", t)
	}
	return nil
}

// writePlain writes plain values (as goccy/go-yaml decodes into any) in flow
// syntax, redacting under the names that say a value is secret.
func writePlain(b *strings.Builder, v any, secret bool, name string, flow bool) {
	switch v := v.(type) {
	case yaml.MapSlice:
		b.WriteByte('{')
		for i, it := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			k := fmt.Sprint(it.Key)
			b.WriteString(mapKey(k) + ": ")
			writePlain(b, it.Value, secret || secretName(k), k, true)
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writePlain(b, e, secret, name, true)
		}
		b.WriteByte(']')
	case string:
		switch {
		case secret && v != "":
			b.WriteString(redacted)
		case dsnName(name):
			b.WriteString(scalar(escapeRefs(redactDsn(v)), flow))
		default:
			b.WriteString(scalar(escapeRefs(v), flow))
		}
	default:
		if secret && v != nil {
			b.WriteString(redacted)
			return
		}
		writeFlow(b, v)
	}
}

// markers adds to out the keys goccy/go-yaml writes, anywhere in v, for an
// embedded struct that holds nothing: a type embedded for its methods, as an
// "Unimplemented..." type is, which goccy writes as a key of its own --
// `unimplementedexporterconfig: {}` -- since it inlines only what is tagged
// `,inline`. Such a key reads back as it was, and says nothing.
func markers(v reflect.Value, out map[string]bool, seen map[uintptr]bool) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		markers(v.Elem(), out, seen)
	case reflect.Interface:
		if !v.IsNil() {
			markers(v.Elem(), out, seen)
		}
	case reflect.Struct:
		t := v.Type()
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() {
				continue
			}
			if et := sf.Type; sf.Anonymous {
				if et.Kind() == reflect.Pointer {
					et = et.Elem()
				}
				if et.Kind() == reflect.Struct && !holdsExported(et) {
					if name, ok := goccyKey(sf); ok {
						out[name] = true
					}
					continue
				}
			}
			markers(v.Field(i), out, seen)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			markers(v.Index(i), out, seen)
		}
	case reflect.Map:
		for it := v.MapRange(); it.Next(); {
			markers(it.Value(), out, seen)
		}
	}
}

// holdsExported reports whether the struct type t has a field goccy writes.
func holdsExported(t reflect.Type) bool {
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			return true
		}
	}
	return false
}

// goccyKey is the key goccy/go-yaml writes the embedded field sf under; ok is
// false for one it does not write as a key of its own.
func goccyKey(sf reflect.StructField) (key string, ok bool) {
	tag := sf.Tag.Get("yaml")
	if tag == "" {
		tag = sf.Tag.Get("json")
	}
	name, opts, _ := strings.Cut(tag, ",")
	if name == "-" || slices.Contains(strings.Split(opts, ","), "inline") {
		return "", false
	}
	if name == "" {
		name = strings.ToLower(sf.Name)
	}
	return name, true
}

// dropMarkers is a without the entries named in names whose value is empty.
func dropMarkers(a any, names map[string]bool) any {
	switch a := a.(type) {
	case yaml.MapSlice:
		out := make(yaml.MapSlice, 0, len(a))
		for _, it := range a {
			if names[fmt.Sprint(it.Key)] && emptyPlain(it.Value) {
				continue
			}
			out = append(out, yaml.MapItem{Key: it.Key, Value: dropMarkers(it.Value, names)})
		}
		return out
	case []any:
		out := make([]any, len(a))
		for i, e := range a {
			out[i] = dropMarkers(e, names)
		}
		return out
	}
	return a
}

// emptyPlain reports whether a, as goccy/go-yaml decodes into any, is null or
// an empty mapping.
func emptyPlain(a any) bool {
	switch a := a.(type) {
	case nil:
		return true
	case yaml.MapSlice:
		return len(a) == 0
	case map[string]any:
		return len(a) == 0
	}
	return false
}

// fieldAt is the field at index of the struct v, through inlined pointers;
// ok is false for a nil one on the way.
func fieldAt(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v, true
}

// holdsSecret reports whether a value of type t holds a Secret anywhere.
func holdsSecret(t reflect.Type) bool {
	return holdsSecretIn(t, map[reflect.Type]bool{})
}

func holdsSecretIn(t reflect.Type, seen map[reflect.Type]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	if isSecret(t) {
		return true
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return holdsSecretIn(t.Elem(), seen)
	case reflect.Map:
		return holdsSecretIn(t.Key(), seen) || holdsSecretIn(t.Elem(), seen)
	case reflect.Struct:
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() && holdsSecretIn(f.Type, seen) {
				return true
			}
		}
	}
	return false
}

// secretName reports whether a value named name is secret, by the name, as
// payday's config command decides.
func secretName(name string) bool {
	k := strings.ToLower(name)
	switch k {
	case "token", "password", "secret", "secrets", "seal", "key", "keys", "credential", "credentials":
		return true
	}
	for _, suffix := range []string{"_token", "_password", "_secret", "_key", "_keys"} {
		if strings.HasSuffix(k, suffix) {
			return true
		}
	}
	return false
}

// dsnName reports whether a value named name is a data source name, whose
// password is redacted: dsn, or a name ending in _dsn, as dev_dsn does.
func dsnName(name string) bool {
	k := strings.ToLower(name)
	return k == "dsn" || strings.HasSuffix(k, "_dsn")
}

// dsnPassword is the password parameter of a DSN of key=value pairs, quoted or
// not: password=x, password = 'a b'.
var dsnPassword = regexp.MustCompile(`(?i)(\bpassword\s*=\s*)('(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"|[^\s&;]*)`)

// redactDsn is a DSN with its password redacted, in the shapes drivers take
// one: in the user information of a URL (postgres://u:pw@h), before the
// address of a MySQL DSN (u:pw@tcp(h)/db), or as a password= parameter. A
// password that is a reference is kept: it says where the password is.
func redactDsn(s string) string {
	s = redactUserinfo(s)
	return dsnPassword.ReplaceAllStringFunc(s, func(m string) string {
		g := dsnPassword.FindStringSubmatch(m)
		pw := strings.Trim(g[2], `'"`)
		if pw == "" || isRef(pw) {
			return m
		}
		return g[1] + redacted
	})
}

// redactUserinfo redacts the password of user:password@ at the start of the
// address of s. The user information ends at the last "@" of the address, as
// net/url reads it, so a password may hold an "@".
func redactUserinfo(s string) string {
	start := 0
	if i := strings.Index(s, "://"); i >= 0 {
		start = i + 3
	} else if strings.ContainsAny(s[:max(0, strings.IndexByte(s, '@'))], " =") {
		// key=value pairs, not user:password@.
		return s
	}
	end := len(s)
	if i := strings.IndexAny(s[start:], "/?#"); i >= 0 && start > 0 {
		end = start + i
	}
	if start == 0 {
		// A MySQL DSN: the address is up to the protocol's "(".
		if i := strings.IndexByte(s, '('); i >= 0 {
			end = i
		}
	}
	at := strings.LastIndexByte(s[start:end], '@')
	if at < 0 {
		return s
	}
	at += start
	colon := strings.IndexByte(s[start:at], ':')
	if colon < 0 {
		return s
	}
	colon += start
	pw := s[colon+1 : at]
	if pw == "" || isRef(pw) {
		return s
	}
	return s[:colon+1] + redacted + s[at:]
}

// escapeRefs writes s so that the file reads it as it is: every "$" doubled.
func escapeRefs(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

// rawScalar is a scalar as it was written, other than a string: 1.10, 0x1F.
type rawScalar string

// The places a scalar is written in the output.
type place int

const (
	blockValue place = iota // after "k: ", followed by a comment
	flowValue               // inside [...] or {...}
	keyName                 // a key, in a block or in {...}
)

// scalar is s as a YAML scalar where it goes, after "k: " or, when flow is
// set, inside [...] or {...}: plain where the parser reads that as the same
// string, quoted otherwise.
func scalar(s string, flow bool) string {
	if s == redacted {
		return s
	}
	at := blockValue
	if flow {
		at = flowValue
	}
	if plainAt(s, at) {
		return s
	}
	return quoted(s)
}

// key is the name of a field as a key of the output.
func key(name string) string {
	if plainAt(name, keyName) {
		return name
	}
	return quoted(name)
}

// mapKey is a key of a map, which the file reads references in.
func mapKey(k string) string {
	return key(escapeRefs(k))
}

// plainAt reports whether the parser reads s, written plain at the place, as
// the string s.
func plainAt(s string, at place) bool {
	if s == "" || strings.ContainsAny(s, "\n\r\t\"'") || s != strings.TrimSpace(s) {
		return false
	}
	switch at {
	case blockValue:
		return readsAs(s, "k: "+s+"  # c", func(n ast.Node) ast.Node { return valueOf(n, "k") })
	case flowValue:
		return readsAs(s, "k: ["+s+"]", func(n ast.Node) ast.Node {
			if seq, ok := valueOf(n, "k").(*ast.SequenceNode); ok && len(seq.Values) == 1 {
				return seq.Values[0]
			}
			return nil
		}) && readsAs(s, "k: {x: "+s+"}", func(n ast.Node) ast.Node {
			return valueOf(valueOf(n, "k"), "x")
		})
	default:
		return readsAs(s, s+": v", keyOf) && readsAs(s, "{"+s+": v}", keyOf)
	}
}

// readsAs parses src and reports whether the node pick picks of it is the
// string s.
func readsAs(s string, src string, pick func(ast.Node) ast.Node) bool {
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil || len(f.Docs) != 1 {
		return false
	}
	sn, ok := pick(f.Docs[0].Body).(*ast.StringNode)
	return ok && sn.Value == s
}

// valueOf is the value of key k of the mapping n, if n is a mapping of it alone.
func valueOf(n ast.Node, k string) ast.Node {
	var mv *ast.MappingValueNode
	switch v := n.(type) {
	case *ast.MappingValueNode:
		mv = v
	case *ast.MappingNode:
		if len(v.Values) != 1 {
			return nil
		}
		mv = v.Values[0]
	default:
		return nil
	}
	if kn, ok := mv.Key.(*ast.StringNode); !ok || kn.Value != k {
		return nil
	}
	return mv.Value
}

// keyOf is the key of the mapping n, if it has one key.
func keyOf(n ast.Node) ast.Node {
	switch v := n.(type) {
	case *ast.MappingValueNode:
		return v.Key
	case *ast.MappingNode:
		if len(v.Values) == 1 {
			return v.Values[0].Key
		}
	}
	return nil
}

// addressable is v, or a copy of it that has an address, for the methods of
// its pointer type.
func addressable(v reflect.Value) reflect.Value {
	if v.CanAddr() {
		return v.Addr()
	}
	c := reflect.New(v.Type())
	c.Elem().Set(v)
	return c
}
