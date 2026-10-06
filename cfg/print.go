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
//   - the password of a value named dsn, in a URL or as password=.
//
// A value read through references, such as `postgres://u:${env:PW}@h`, prints
// as it was written: it says where the secret is, not what it is.
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
		case len(c.children) > 0:
			b.WriteString(indent + key(c.name) + ":\n")
			c.render(b, depth+1)
		case c.empty != "":
			b.WriteString(indent + key(c.name) + ": " + c.empty + "\n")
		}
	}
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
		if o.Source == Unset && v.IsZero() {
			continue
		}
		val, err := p.leaf(f, v, name)
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
			if o := p.origin(f); o.Source != Unset {
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

func (p *printer) leaf(f *field, v reflect.Value, name string) (string, error) {
	secret := f.secret || secretName(name)
	if w, ok := p.written[f.key]; ok && !holdsSecret(f.typ) {
		// Read through references: as written.
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
		case strings.EqualFold(name, "dsn"):
			b.WriteString(scalar(redactDsn(v), flow))
		default:
			b.WriteString(scalar(v, flow))
		}
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
			b.WriteString(scalar(k, true) + ": ")
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
	if t == bytesType {
		if secret {
			b.WriteString(redacted)
			return nil
		}
		b.WriteString("!!binary " + base64.StdEncoding.EncodeToString(v.Bytes()))
		return nil
	}
	if tm, ok := v.Interface().(encoding.TextMarshaler); ok {
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
		out, err := yaml.Marshal(v.Interface())
		if err != nil {
			return err
		}
		var a any
		if err := yaml.UnmarshalWithOptions(out, &a, yaml.UseOrderedMap()); err != nil {
			return err
		}
		writePlain(b, a, secret, name, flow)
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		s := v.String()
		if strings.EqualFold(name, "dsn") {
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
			b.WriteString(scalar(it.key, true) + ": ")
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
			b.WriteString(scalar(f.name, true) + ": ")
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
			b.WriteString(scalar(k, true) + ": ")
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
		case strings.EqualFold(name, "dsn"):
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

var (
	dsnUserinfo = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://[^:/@]*:)([^@/]*)(@)`)
	dsnPassword = regexp.MustCompile(`(?i)(password=)([^ &]*)`)
)

// redactDsn is a DSN with its password redacted, in either of the two shapes a
// driver takes one: in the URL's user information, or as a password=
// parameter. A password that is a reference is kept: it says where the
// password is.
func redactDsn(s string) string {
	keep := func(re *regexp.Regexp) func(string) string {
		return func(m string) string {
			g := re.FindStringSubmatch(m)
			if g[2] == "" || isRef(g[2]) {
				return m
			}
			return g[1] + redacted + strings.Join(g[3:], "")
		}
	}
	s = dsnUserinfo.ReplaceAllStringFunc(s, keep(dsnUserinfo))
	return dsnPassword.ReplaceAllStringFunc(s, keep(dsnPassword))
}

// escapeRefs writes s so that the file reads it as it is: every "$" doubled.
func escapeRefs(s string) string {
	return strings.ReplaceAll(s, "$", "$$")
}

// scalar is s as a YAML scalar where it goes, after "k: " or, when flow is
// set, inside [...] or {...}: plain where the parser reads that as the same
// string, quoted otherwise.
func scalar(s string, flow bool) string {
	if s == redacted {
		return s
	}
	if plainAs(s, flow) {
		return s
	}
	return quoted(s)
}

func plainAs(s string, flow bool) bool {
	if s == "" || strings.ContainsAny(s, "\n\r\t\"'") || s != strings.TrimSpace(s) {
		return false
	}
	src := "k: " + s
	if flow {
		src = "k: [" + s + "]"
	}
	f, err := parser.ParseBytes([]byte(src), 0)
	if err != nil || len(f.Docs) != 1 {
		return false
	}
	var v ast.Node
	switch n := f.Docs[0].Body.(type) {
	case *ast.MappingValueNode:
		v = n.Value
	case *ast.MappingNode:
		if len(n.Values) != 1 {
			return false
		}
		v = n.Values[0].Value
	default:
		return false
	}
	if flow {
		seq, ok := v.(*ast.SequenceNode)
		if !ok || len(seq.Values) != 1 {
			return false
		}
		v = seq.Values[0]
	}
	sn, ok := v.(*ast.StringNode)
	return ok && sn.Value == s
}

// key is the name of a field as a key of the output.
func key(name string) string {
	if plainAs(name, true) {
		return name
	}
	return quoted(name)
}
