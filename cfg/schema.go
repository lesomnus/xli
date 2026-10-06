package cfg

import (
	"encoding"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml"
)

// field is one leaf of the configuration struct: a value that is read as a
// whole from a file, a variable or a flag.
type field struct {
	key   string // dotted path, e.g. "ldap.addr"
	env   string // environment variable, "" if none
	index []int  // reflect field indices from the root, through pointers
	typ   reflect.Type
	// secret reports that the value is redacted when printed.
	secret bool
	// group reports a struct of leaves rather than a leaf; a flag may be bound
	// to one to set it whole.
	group bool
}

// covers reports whether f is g or a group g is in.
func (f *field) covers(g *field) bool {
	return f == g || (f.group && strings.HasPrefix(g.key, f.key+"."))
}

// schema is the set of leaves of a struct type, in declaration order.
type schema struct {
	root   reflect.Type
	fields []*field
	groups []*field
	// all are the groups and the fields, a group before what is in it.
	all   []*field
	byKey map[string]*field
	byEnv map[string]*field
}

func newSchema(t reflect.Type, prefix string) (*schema, error) {
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cfg: configuration must be a struct, not %s", t)
	}

	s := &schema{
		root:  t,
		byKey: map[string]*field{},
		byEnv: map[string]*field{},
	}
	if err := s.walk(t, prefix, nil, nil, false, map[reflect.Type]bool{}, map[string]bool{}); err != nil {
		return nil, err
	}
	return s, nil
}

// walk adds the fields of t to s. secret is set in a block tagged secret,
// which makes every field in it one. taken are the keys of the fields and the
// blocks so far, which two inlined structs may both have.
func (s *schema) walk(t reflect.Type, prefix string, path []string, index []int, secret bool, seen map[reflect.Type]bool, taken map[string]bool) error {
	if seen[t] {
		// A recursive type has no finite set of leaves.
		return fmt.Errorf("cfg: %s refers to itself", t)
	}
	seen[t] = true
	defer delete(seen, t)

	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag, ok := parseTag(sf)
		if !ok {
			continue
		}

		ft := sf.Type
		if ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Pointer {
			return fmt.Errorf("cfg: %s.%s: a pointer to a pointer cannot be read", t, sf.Name)
		}
		if strings.HasPrefix(tag.name, "x-") {
			return fmt.Errorf("cfg: %s.%s: %q is never read from a file, where x- keys are ignored", t, sf.Name, tag.name)
		}
		if tag.env != "" && tag.env != "-" && !validEnvName(tag.env) {
			return fmt.Errorf("cfg: %s.%s: env:%q is not a variable name", t, sf.Name, tag.env)
		}
		idx := append(append([]int{}, index...), i)
		if tag.inline {
			st := ft
			if st.Kind() == reflect.Pointer {
				st = st.Elem()
			}
			if st.Kind() != reflect.Struct {
				return fmt.Errorf("cfg: %s.%s: only a struct can be inlined", t, sf.Name)
			}
			if err := s.walk(st, prefix, path, idx, secret || tag.secret, seen, taken); err != nil {
				return err
			}
			continue
		}

		p := append(append([]string{}, path...), tag.name)
		key := strings.Join(p, ".")
		if taken[key] {
			return fmt.Errorf("cfg: %s: two fields are named %q", t, key)
		}
		taken[key] = true

		if !isLeaf(ft) {
			st := ft
			if st.Kind() == reflect.Pointer {
				st = st.Elem()
			}
			g := &field{key: key, index: idx, typ: ft, group: true}
			s.groups = append(s.groups, g)
			s.all = append(s.all, g)
			if err := s.walk(st, prefix, p, idx, secret || tag.secret, seen, taken); err != nil {
				return err
			}
			continue
		}

		f := &field{
			key:    key,
			index:  idx,
			typ:    ft,
			secret: secret || tag.secret || isSecret(ft),
		}
		switch tag.env {
		case "-":
		case "":
			if prefix != "" {
				f.env = envName(prefix, p)
			}
		default:
			f.env = tag.env
		}

		if f.env != "" {
			if g, ok := s.byEnv[f.env]; ok {
				return fmt.Errorf("cfg: %s and %s are both read from %s", g.key, f.key, f.env)
			}
			s.byEnv[f.env] = f
		}
		s.byKey[f.key] = f
		s.fields = append(s.fields, f)
		s.all = append(s.all, f)
	}
	return nil
}

type tag struct {
	name   string
	env    string
	inline bool
	secret bool
}

// parseTag reads how a field is named. ok is false for a field that is left
// out.
func parseTag(sf reflect.StructField) (tag, bool) {
	var t tag
	for _, key := range []string{"cfg", "yaml", "json"} {
		v, ok := sf.Tag.Lookup(key)
		if !ok {
			continue
		}
		name, opts, _ := strings.Cut(v, ",")
		if name == "-" && opts == "" {
			if t.name != "" {
				// Named by a tag that comes first, e.g. `cfg:"x" yaml:"-"`.
				continue
			}
			return t, false
		}
		for _, o := range strings.Split(opts, ",") {
			switch o {
			case "inline":
				t.inline = true
			case "secret":
				if key == "cfg" {
					t.secret = true
				}
			}
		}
		if t.name == "" {
			t.name = name
		}
	}
	if t.name == "" {
		st := sf.Type
		if st.Kind() == reflect.Pointer {
			st = st.Elem()
		}
		if sf.Anonymous && st.Kind() == reflect.Struct && !readsItself(st) {
			// As encoding/json does: an embedded struct's fields are the
			// holder's; anything else embedded is a field named after its type.
			t.inline = true
		}
		t.name = snake(sf.Name)
	}
	t.env = sf.Tag.Get("env")
	return t, true
}

// snake turns a Go field name into snake_case: ListenAddr is listen_addr,
// AddrTLS is addr_tls and TLSConfig is tls_config.
func snake(s string) string {
	rs := []rune(s)
	b := strings.Builder{}
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rs[i-1]
				next := i+1 < len(rs) && unicode.IsLower(rs[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && next) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// envPrefix is an application name as the start of an environment variable:
// "go-app" is "GO_APP".
func envPrefix(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return '_'
		}
	}, name)
}

func envName(prefix string, path []string) string {
	return prefix + "_" + envPrefix(strings.Join(path, "_"))
}

var (
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	yamlBytes       = reflect.TypeFor[yaml.BytesUnmarshaler]()
	yamlInterface   = reflect.TypeFor[yaml.InterfaceUnmarshaler]()
	nodeUnmarshaler = reflect.TypeFor[yaml.NodeUnmarshaler]()
	secretType      = reflect.TypeFor[secretField]()
)

// isLeaf reports whether a value of type t is read as a whole: anything that is
// not a struct, and a struct that reads itself.
func isLeaf(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return true
	}
	return readsItself(t)
}

// readsItself reports whether a value of type t (or a pointer to one) decodes
// itself.
func readsItself(t reflect.Type) bool {
	p := reflect.PointerTo(t)
	for _, i := range []reflect.Type{secretType, textUnmarshaler, yamlBytes, yamlInterface, nodeUnmarshaler} {
		if t.Implements(i) || p.Implements(i) {
			return true
		}
	}
	return false
}

func isSecret(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return reflect.PointerTo(t).Implements(secretType)
}

// value is the field f of root, allocating the pointers on the way when alloc is
// set. ok is false when a pointer on the way is nil and alloc is not set.
func (f *field) value(root reflect.Value, alloc bool) (reflect.Value, bool) {
	v := root
	for _, i := range f.index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !alloc {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v, true
}

// find is the leaf of root that ptr points to.
func (s *schema) find(root reflect.Value, ptr any) (*field, bool) {
	return s.findIn(s.fields, root, ptr)
}

// findAny is the leaf or the group of root that ptr points to.
func (s *schema) findAny(root reflect.Value, ptr any) (*field, bool) {
	if f, ok := s.find(root, ptr); ok {
		return f, true
	}
	return s.findIn(s.groups, root, ptr)
}

func (s *schema) findIn(fields []*field, root reflect.Value, ptr any) (*field, bool) {
	pv := reflect.ValueOf(ptr)
	if pv.Kind() != reflect.Pointer || pv.IsNil() {
		return nil, false
	}
	for _, f := range fields {
		v, ok := f.value(root, false)
		if !ok {
			continue
		}
		if v.Addr().Pointer() == pv.Pointer() && v.Type() == pv.Type().Elem() {
			return f, true
		}
	}
	return nil, false
}
