package cfg

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// decoder reads a YAML document into a struct, walking the struct rather than
// handing the document to a YAML decoder, so that the names, the strictness and
// the references are cfg's own.
type decoder struct {
	r       *resolver
	file    string
	anchors map[string]ast.Node

	// leaf, when set, is called for every field of the root struct that is a
	// leaf of the schema, with the node its value was read from.
	leaf func(key string, n ast.Node, refs []string, cleared bool)
}

// body is the single document of f, or nil for an empty file.
func (d *decoder) body(f *ast.File) (ast.Node, error) {
	var body ast.Node
	for _, doc := range f.Docs {
		if doc.Body == nil {
			continue
		}
		if body != nil {
			return nil, errors.New("more than one document")
		}
		body = doc.Body
	}

	d.anchors = map[string]ast.Node{}
	if body != nil {
		for _, n := range ast.Filter(ast.AnchorType, body) {
			a := n.(*ast.AnchorNode)
			d.anchors[a.Name.GetToken().Value] = a.Value
		}
	}
	return body, nil
}

// resolve follows anchors, aliases and tags to the node that holds the value.
// str reports a `!!str` tag.
func (d *decoder) resolve(n ast.Node) (_ ast.Node, str bool, err error) {
	for range 64 {
		switch v := n.(type) {
		case *ast.AnchorNode:
			n = v.Value
		case *ast.AliasNode:
			name := v.Value.GetToken().Value
			a, ok := d.anchors[name]
			if !ok {
				return nil, false, fmt.Errorf("alias *%s names no anchor", name)
			}
			n = a
		case *ast.TagNode:
			if v.Start.Value == "!!str" {
				str = true
			}
			n = v.Value
		default:
			return n, str, nil
		}
	}
	return nil, false, errors.New("aliases nest too deeply")
}

// at is the origin of a node in the file, for an error about it.
func (d *decoder) at(n ast.Node, key string) Origin {
	o := Origin{Source: File, Key: key, Name: d.file}
	if n != nil {
		if t := n.GetToken(); t != nil && t.Position != nil {
			o.Line, o.Column = t.Position.Line, t.Position.Column
		}
	}
	return o
}

// decode reads n into v. key is the dotted path of v from the root struct, and
// root reports that v is still on the root struct's own fields (not inside a
// list or a map), where leaves are reported to d.leaf.
func (d *decoder) decode(n ast.Node, v reflect.Value, key string, root bool) (es errs) {
	n, str, err := d.resolve(n)
	if err != nil {
		es.add(d.at(n, key), err)
		return es
	}

	leaf := root && isLeaf(v.Type())
	if _, ok := n.(*ast.NullNode); ok || n == nil {
		v.SetZero()
		if leaf && d.leaf != nil {
			d.leaf(key, n, nil, true)
		}
		return nil
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return d.decode(n, v.Elem(), key, root)
	}

	var refs []string
	if leaf && d.leaf != nil {
		d.r.refs = nil
		defer func() {
			if len(es) == 0 {
				d.leaf(key, n, refs, false)
			}
		}()
	}

	ptr := v.Addr().Interface()
	if sf, ok := ptr.(secretField); ok {
		text, _, ok := scalarText(n)
		if !ok {
			es.addf(d.at(n, key), "want a string")
			return es
		}
		if err := sf.setSecret(text, d.r); err != nil {
			es.add(d.at(n, key), err)
		}
		refs = sf.refs()
		return es
	}
	switch ptr.(type) {
	case yaml.BytesUnmarshaler, yaml.InterfaceUnmarshaler, yaml.NodeUnmarshaler:
		if err := yaml.NodeToValue(n, ptr); err != nil {
			es.add(d.at(n, key), err)
		}
		return es
	case encoding.TextUnmarshaler:
		text, err := d.text(n, str)
		if err == nil {
			err = ptr.(encoding.TextUnmarshaler).UnmarshalText([]byte(text))
		}
		if err != nil {
			es.add(d.at(n, key), err)
		}
		refs = d.r.refs
		return es
	}

	switch v.Kind() {
	case reflect.Struct:
		es = d.decodeStruct(n, v, key, root)
	case reflect.Slice, reflect.Array:
		es = d.decodeList(n, v, key)
	case reflect.Map:
		es = d.decodeMap(n, v, key)
	case reflect.Interface:
		if err := yaml.NodeToValue(n, ptr); err != nil {
			es.add(d.at(n, key), err)
		}
	default:
		text, err := d.text(n, str)
		if err == nil {
			err = setScalar(v, text)
		}
		if err != nil {
			es.add(d.at(n, key), err)
		}
	}
	refs = d.r.refs
	return es
}

// text is the text of a scalar node with its references resolved.
func (d *decoder) text(n ast.Node, str bool) (string, error) {
	text, isString, ok := scalarText(n)
	if !ok {
		return "", fmt.Errorf("want a single value, not %s", nodeKind(n))
	}
	if isString || str {
		return d.r.expand(text)
	}
	return text, nil
}

// scalarText is the text of a scalar: a string as decoded (quotes and escapes
// resolved), anything else as written, so that `1.10` stays "1.10".
func scalarText(n ast.Node) (text string, isString bool, ok bool) {
	switch v := n.(type) {
	case *ast.StringNode:
		return v.Value, true, true
	case *ast.LiteralNode:
		return v.Value.Value, true, true
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode, *ast.InfinityNode, *ast.NanNode:
		return n.GetToken().Value, false, true
	}
	return "", false, false
}

func nodeKind(n ast.Node) string {
	switch n.(type) {
	case *ast.MappingNode, *ast.MappingValueNode:
		return "a mapping"
	case *ast.SequenceNode:
		return "a list"
	default:
		return n.Type().String()
	}
}

// entry is one key and value of a mapping.
type entry struct {
	key   ast.Node
	value ast.Node
}

// entries are the keys and values of a mapping, those of merge keys (`<<`)
// first so the mapping's own override them.
func (d *decoder) entries(n ast.Node) ([]entry, error) {
	var mvs []*ast.MappingValueNode
	switch v := n.(type) {
	case *ast.MappingNode:
		mvs = v.Values
	case *ast.MappingValueNode:
		mvs = []*ast.MappingValueNode{v}
	default:
		return nil, fmt.Errorf("want a mapping, not %s", nodeKind(n))
	}

	merged := []entry{}
	own := []entry{}
	for _, mv := range mvs {
		if _, ok := mv.Key.(*ast.MergeKeyNode); !ok {
			own = append(own, entry{mv.Key, mv.Value})
			continue
		}

		src, _, err := d.resolve(mv.Value)
		if err != nil {
			return nil, err
		}
		srcs := []ast.Node{src}
		if seq, ok := src.(*ast.SequenceNode); ok {
			srcs = seq.Values
		}
		for _, s := range srcs {
			s, _, err := d.resolve(s)
			if err != nil {
				return nil, err
			}
			es, err := d.entries(s)
			if err != nil {
				return nil, fmt.Errorf("<<: %w", err)
			}
			merged = append(merged, es...)
		}
	}
	return append(merged, own...), nil
}

// fieldOf is a struct field by the name it has in the configuration, through
// inlined structs.
type structField struct {
	index []int
}

func structFields(t reflect.Type) map[string]structField {
	fs := map[string]structField{}
	var walk func(t reflect.Type, index []int)
	walk = func(t reflect.Type, index []int) {
		for i := range t.NumField() {
			sf := t.Field(i)
			if !sf.IsExported() {
				continue
			}
			tag, ok := parseTag(sf)
			if !ok {
				continue
			}
			idx := append(append([]int{}, index...), i)
			if tag.inline {
				st := sf.Type
				if st.Kind() == reflect.Pointer {
					st = st.Elem()
				}
				if st.Kind() == reflect.Struct {
					walk(st, idx)
				}
				continue
			}
			if _, ok := fs[tag.name]; !ok {
				fs[tag.name] = structField{index: idx}
			}
		}
	}
	walk(t, nil)
	return fs
}

// fieldByIndex is the field at index, allocating pointers to inlined structs.
func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v
}

func (d *decoder) decodeStruct(n ast.Node, v reflect.Value, key string, root bool) (es errs) {
	entries, err := d.entries(n)
	if err != nil {
		es.add(d.at(n, key), err)
		return es
	}

	fs := structFields(v.Type())
	for _, e := range entries {
		k, _, err := d.resolve(e.key)
		if err != nil {
			es.add(d.at(e.key, key), err)
			continue
		}
		name, _, ok := scalarText(k)
		if !ok {
			es.addf(d.at(e.key, key), "want a name as the key")
			continue
		}
		if strings.HasPrefix(name, "x-") {
			continue
		}

		sub := name
		if key != "" {
			sub = key + "." + name
		}
		f, ok := fs[name]
		if !ok {
			names := make([]string, 0, len(fs))
			for n := range fs {
				names = append(names, n)
			}
			slices.Sort(names)
			es.add(d.at(e.key, sub), fmt.Errorf("%w%s", errUnknownKey, hint(name, names)))
			continue
		}
		es = append(es, d.decode(e.value, fieldByIndex(v, f.index), sub, root)...)
	}
	return es
}

var errUnknownKey = errors.New("nothing reads this key")

func (d *decoder) decodeList(n ast.Node, v reflect.Value, key string) (es errs) {
	seq, ok := n.(*ast.SequenceNode)
	if !ok {
		es.addf(d.at(n, key), "want a list, not %s", nodeKind(n))
		return es
	}

	if v.Kind() == reflect.Array {
		if len(seq.Values) != v.Len() {
			es.addf(d.at(n, key), "want %d values, got %d", v.Len(), len(seq.Values))
			return es
		}
		for i, e := range seq.Values {
			es = append(es, d.decode(e, v.Index(i), fmt.Sprintf("%s[%d]", key, i), false)...)
		}
		return es
	}

	l := reflect.MakeSlice(v.Type(), len(seq.Values), len(seq.Values))
	for i, e := range seq.Values {
		es = append(es, d.decode(e, l.Index(i), fmt.Sprintf("%s[%d]", key, i), false)...)
	}
	v.Set(l)
	return es
}

func (d *decoder) decodeMap(n ast.Node, v reflect.Value, key string) (es errs) {
	entries, err := d.entries(n)
	if err != nil {
		es.add(d.at(n, key), err)
		return es
	}

	m := reflect.MakeMapWithSize(v.Type(), len(entries))
	for _, e := range entries {
		kn, str, err := d.resolve(e.key)
		if err != nil {
			es.add(d.at(e.key, key), err)
			continue
		}
		name, err := d.text(kn, str)
		if err != nil {
			es.add(d.at(e.key, key), err)
			continue
		}

		sub := key + "." + name
		k := reflect.New(v.Type().Key()).Elem()
		if err := setText(k, name, d.r); err != nil {
			es.add(d.at(e.key, sub), err)
			continue
		}
		val := reflect.New(v.Type().Elem()).Elem()
		if sub := d.decode(e.value, val, sub, false); len(sub) > 0 {
			es = append(es, sub...)
			continue
		}
		m.SetMapIndex(k, val)
	}
	v.Set(m)
	return es
}
