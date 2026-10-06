package cfg

import (
	"encoding"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
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
	anchors map[string][]anchor

	// ordered makes anyOf give a mapping as a yaml.MapSlice, its keys in order
	// and of their own types, for a type that decodes itself.
	ordered bool
	// raw makes anyOf give a scalar other than a string as written, a
	// rawScalar, for Print.
	raw bool

	// active are the nodes being read: an alias to one of them is an alias to
	// a value it is in, which has no end.
	active map[ast.Node]bool
	// nodes counts the nodes read, which aliases can multiply.
	nodes int

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
		if _, ok := doc.Body.(*ast.DirectiveNode); ok {
			// `%YAML 1.2` before the document.
			continue
		}
		if body != nil {
			return nil, errors.New("more than one document")
		}
		body = doc.Body
	}

	d.anchors = map[string][]anchor{}
	if body != nil {
		for _, n := range ast.Filter(ast.AnchorType, body) {
			a := n.(*ast.AnchorNode)
			name := a.Name.GetToken().Value
			d.anchors[name] = append(d.anchors[name], anchor{offset(a), a.Value})
		}
	}
	return body, nil
}

// anchor is a node an alias may name, and where it is defined.
type anchor struct {
	at   int
	node ast.Node
}

func offset(n ast.Node) int {
	if t := n.GetToken(); t != nil && t.Position != nil {
		return t.Position.Offset
	}
	return 0
}

// anchored is the node the alias at `at` names: the nearest anchor of that
// name defined before it, as an anchor may be defined again.
func (d *decoder) anchored(name string, at int) (ast.Node, bool) {
	var found ast.Node
	for _, a := range d.anchors[name] {
		if a.at < at {
			found = a.node
		}
	}
	return found, found != nil
}

// resolve follows anchors, aliases and tags to the node that holds the value.
// tag is the tag it was given, such as "!!str", or "".
func (d *decoder) resolve(n ast.Node) (_ ast.Node, tag string, err error) {
	for range 64 {
		switch v := n.(type) {
		case *ast.AnchorNode:
			n = v.Value
		case *ast.AliasNode:
			name := v.Value.GetToken().Value
			a, ok := d.anchored(name, offset(v))
			if !ok {
				return nil, "", fmt.Errorf("alias *%s names no anchor", name)
			}
			n = a
		case *ast.TagNode:
			if tag == "" {
				tag = v.Start.Value
			}
			n = v.Value
		default:
			return n, tag, nil
		}
	}
	return nil, "", errors.New("aliases nest too deeply")
}

// maxNodes is the most nodes a document is read as. A configuration has a few
// hundred; aliases of aliases can make billions of a few lines.
const maxNodes = 1 << 20

// enter marks n as being read, until leave. It fails for a node that is being
// read already, which only an alias to a value it is in can make happen.
func (d *decoder) enter(n ast.Node) error {
	if d.active == nil {
		d.active = map[ast.Node]bool{}
	}
	if d.active[n] {
		return errors.New("an alias refers to a value it is in")
	}
	if d.nodes++; d.nodes > maxNodes {
		return errors.New("aliases expand to too many values")
	}
	d.active[n] = true
	return nil
}

func (d *decoder) leave(n ast.Node) {
	delete(d.active, n)
}

func isNull(n ast.Node) bool {
	_, ok := n.(*ast.NullNode)
	return ok || n == nil
}

// at is the origin of a node in the file, for an error about it.
func (d *decoder) at(n ast.Node, key string) Origin {
	o := Origin{Source: SourceFile, Key: key, Name: d.file}
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
	written := n
	n, tag, err := d.resolve(n)
	if err != nil {
		es.add(d.at(written, key), err)
		return es
	}
	str := tag == "!!str"

	leaf := root && isLeaf(v.Type())
	if isNull(n) && !str {
		v.SetZero()
		if root && d.leaf != nil {
			// A leaf, or a block whose leaves are all cleared.
			d.leaf(key, n, nil, true)
		}
		return nil
	}

	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return d.decode(written, v.Elem(), key, root)
	}

	if err := d.enter(n); err != nil {
		es.add(d.at(written, key), err)
		return es
	}
	defer d.leave(n)

	var refs []string
	if leaf && d.leaf != nil {
		d.r.refs = nil
		defer func() {
			if onlyPending(es) {
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
	if tag == "!!binary" && v.Type() == bytesType {
		text, _, ok := scalarText(n)
		b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(text), ""))
		if !ok || err != nil {
			es.addf(d.at(n, key), "want base64 for !!binary")
			return es
		}
		v.SetBytes(b)
		return es
	}
	// A value that reads itself is read whole, into a new one: the default it
	// replaces may share what it holds with the defaults.
	fresh := reflect.New(v.Type())
	switch ptr.(type) {
	case yaml.BytesUnmarshaler, yaml.InterfaceUnmarshaler, yaml.NodeUnmarshaler:
		// The type decodes itself, and would see neither the references nor
		// the anchors outside its block: it is given the block as plain values,
		// with references resolved and aliases and merges expanded.
		if err := d.unmarshal(n, str, fresh.Interface()); err != nil {
			es.add(d.at(n, key), err)
			return es
		}
		v.Set(fresh.Elem())
		refs = d.r.refs
		return es
	case encoding.TextUnmarshaler:
		text, err := d.text(n, str)
		if err == nil {
			err = fresh.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(text))
		}
		if err != nil {
			es.add(d.at(n, key), err)
			return es
		}
		v.Set(fresh.Elem())
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
		if v.NumMethod() != 0 {
			if err := yaml.NodeToValue(n, ptr); err != nil {
				es.add(d.at(n, key), err)
			}
			break
		}
		a, err := d.anyValue(n, str, 0)
		if err != nil {
			es.add(d.at(n, key), err)
			break
		}
		if a == nil {
			v.SetZero()
		} else {
			v.Set(reflect.ValueOf(a))
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

// asWritten is n as plain values with its references not resolved: what the
// file says, for Print.
func (d *decoder) asWritten(n ast.Node) any {
	w := &decoder{r: &resolver{verbatim: true}, anchors: d.anchors, raw: true}
	a, err := w.anyOf(n, false, 0)
	if err != nil {
		return nil
	}
	return a
}

// unmarshal decodes n into ptr, a type that decodes itself, by handing it n as
// plain values: with the references in its strings resolved, and its aliases
// and merges expanded.
func (d *decoder) unmarshal(n ast.Node, str bool, ptr any) error {
	prev, keep := d.ordered, d.r.keepFiles
	d.ordered, d.r.keepFiles = true, true
	a, err := d.anyValue(n, str, 0)
	d.ordered, d.r.keepFiles = prev, keep
	if err != nil {
		return err
	}
	return yaml.Unmarshal([]byte(flowOf(a)), ptr)
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
	case *ast.NullNode:
		// Only asked for under `!!str`, which makes `~` the string "~".
		if t := n.GetToken(); t != nil {
			return t.Value, false, true
		}
		return "", false, true
	}
	return "", false, false
}

var bytesType = reflect.TypeFor[[]byte]()

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
	// merged reports an entry of a merge key's mapping.
	merged bool
}

// entries are the keys and values of a mapping, with those of its merge keys
// (`<<`) as the merge key is specified: a key of the mapping's own overrides a
// merged one, and a mapping earlier in a list of merged ones overrides a later
// one. A value overridden is replaced whole; blocks are not merged deeply.
func (d *decoder) entries(n ast.Node) ([]entry, error) {
	return d.entriesAt(n, 0)
}

func (d *decoder) entriesAt(n ast.Node, depth int) ([]entry, error) {
	if depth > 64 {
		return nil, errors.New("merge keys nest too deeply (a merge that includes itself?)")
	}
	var mvs []*ast.MappingValueNode
	switch v := n.(type) {
	case *ast.MappingNode:
		mvs = v.Values
	case *ast.MappingValueNode:
		mvs = []*ast.MappingValueNode{v}
	default:
		return nil, fmt.Errorf("want a mapping, not %s", nodeKind(n))
	}
	if d.nodes += len(mvs); d.nodes > maxNodes {
		return nil, errors.New("aliases expand to too many values")
	}

	merged := [][]entry{}
	own := []entry{}
	for _, mv := range mvs {
		if _, ok := mv.Key.(*ast.MergeKeyNode); !ok {
			own = append(own, entry{key: mv.Key, value: mv.Value})
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
			es, err := d.entriesAt(s, depth+1)
			if err != nil {
				return nil, fmt.Errorf("<<: %w", err)
			}
			merged = append(merged, es)
		}
	}
	if len(merged) == 0 {
		return own, nil
	}

	seen := map[string]bool{}
	for _, e := range own {
		if k, ok := d.keyOf(e.key); ok {
			seen[k] = true
		}
	}
	es := []entry{}
	for _, src := range merged {
		for _, e := range src {
			if k, ok := d.keyOf(e.key); ok {
				if seen[k] {
					continue
				}
				seen[k] = true
			}
			e.merged = true
			es = append(es, e)
		}
	}
	return append(es, own...), nil
}

// keyOf is the text of a mapping key; ok is false for a key that is not a
// single value.
func (d *decoder) keyOf(n ast.Node) (string, bool) {
	k, _, err := d.resolve(n)
	if err != nil {
		return "", false
	}
	text, _, ok := scalarText(k)
	return text, ok
}

// structField is a field of a struct by the name it has in the configuration,
// through inlined structs.
type structField struct {
	name   string
	index  []int
	secret bool // tagged `cfg:",secret"`
}

// fieldsOf are the fields of t in order.
func fieldsOf(t reflect.Type) []structField {
	fs := []structField{}
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
			fs = append(fs, structField{name: tag.name, index: idx, secret: tag.secret})
		}
	}
	walk(t, nil)
	return fs
}

func structFields(t reflect.Type) map[string]structField {
	m := map[string]structField{}
	for _, f := range fieldsOf(t) {
		if _, ok := m[f.name]; !ok {
			m[f.name] = f
		}
	}
	return m
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
	merged := map[any]bool{}
	for _, e := range entries {
		kn, tag, err := d.resolve(e.key)
		if err != nil {
			es.add(d.at(e.key, key), err)
			continue
		}
		name, err := d.text(kn, tag == "!!str")
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
		if m.MapIndex(k).IsValid() {
			// Keys written differently that read the same, e.g. 1 and 01:
			// as the merge key is specified, the mapping's own over a
			// merged one, and an earlier merged one over a later one.
			switch {
			case e.merged:
				continue
			case !merged[k.Interface()]:
				es.addf(d.at(e.key, sub), "given twice")
				continue
			}
		}
		val := reflect.New(v.Type().Elem()).Elem()
		if sub := d.decode(e.value, val, sub, false); len(sub) > 0 {
			es = append(es, sub...)
			if !onlyPending(sub) {
				continue
			}
		}
		m.SetMapIndex(k, val)
		merged[k.Interface()] = e.merged
	}
	v.Set(m)
	return es
}

// anyOf is a node as plain Go values (map[string]any, []any, string, numbers,
// bool, nil) with the references in its strings resolved.
func (d *decoder) anyOf(n ast.Node, str bool, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("nests too deeply")
	}
	n, tag, err := d.resolve(n)
	if err != nil {
		return nil, err
	}
	str = str || tag == "!!str"
	if isNull(n) && !str {
		return nil, nil
	}
	if err := d.enter(n); err != nil {
		return nil, err
	}
	defer d.leave(n)
	return d.anyValue(n, str, depth)
}

// anyValue is anyOf for a node resolved and entered.
func (d *decoder) anyValue(n ast.Node, str bool, depth int) (any, error) {
	switch v := n.(type) {
	case *ast.MappingNode, *ast.MappingValueNode:
		es, err := d.entries(v)
		if err != nil {
			return nil, err
		}
		m := map[string]any{}
		ms := yaml.MapSlice{}
		for _, e := range es {
			k, tag, err := d.resolve(e.key)
			if err != nil {
				return nil, err
			}
			name, err := d.text(k, tag == "!!str")
			if err != nil {
				return nil, err
			}
			val, err := d.anyOf(e.value, false, depth+1)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if !d.ordered {
				m[name] = val
				continue
			}
			var kv any = name
			if _, isString, _ := scalarText(k); !isString && tag != "!!str" {
				kv = scalarOf(k, name)
			}
			ms = append(ms, yaml.MapItem{Key: kv, Value: val})
		}
		if d.ordered {
			return ms, nil
		}
		return m, nil
	case *ast.SequenceNode:
		l := make([]any, len(v.Values))
		for i, e := range v.Values {
			val, err := d.anyOf(e, false, depth+1)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			l[i] = val
		}
		return l, nil
	case *ast.StringNode, *ast.LiteralNode:
		return d.text(n, true)
	}
	if str {
		return d.text(n, true)
	}
	if text, _, ok := scalarText(n); ok {
		if d.raw {
			return rawScalar(text), nil
		}
		return scalarOf(n, text), nil
	}
	var a any
	if err := yaml.NodeToValue(n, &a); err != nil {
		return nil, err
	}
	return a, nil
}

// scalarOf is a number or a boolean as YAML 1.2 reads it, as the fields of
// other types do (`010` is ten), in the types goccy/go-yaml gives: uint64 for an
// integer that is not negative, int64 for one that is, float64 and bool. What
// YAML 1.2 does not read as one, such as `1_000`, is the string written.
func scalarOf(n ast.Node, text string) any {
	switch n.(type) {
	case *ast.IntegerNode:
		if strings.Contains(text, "_") {
			break
		}
		if !strings.HasPrefix(text, "-") {
			if u, err := strconv.ParseUint(strings.TrimPrefix(text, "+"), intBase(text), 64); err == nil {
				return u
			}
		} else if i, err := strconv.ParseInt(text, intBase(text), 64); err == nil {
			return i
		}
	case *ast.FloatNode, *ast.InfinityNode, *ast.NanNode:
		if f, err := parseFloat(text, 64); err == nil {
			return f
		}
	case *ast.BoolNode:
		if b, err := strconv.ParseBool(text); err == nil {
			return b
		}
	}
	return text
}

// onlyPending reports whether es holds nothing but warnings about secret files
// not there yet.
func onlyPending(es errs) bool {
	for _, err := range es {
		if !isPending(err) {
			return false
		}
	}
	return true
}
