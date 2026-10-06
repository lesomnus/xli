package cfg

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"
)

// Resolver reads the value a reference of a registered scheme names:
// `${vault:secret/db#password}` calls the "vault" resolver with
// "secret/db#password".
type Resolver func(ref string) ([]byte, error)

// resolver resolves references for one load.
type resolver struct {
	lookup  func(string) (string, bool)
	schemes map[string]Resolver
	// verbatim is set for values from the environment and flags, which are
	// taken as they are: only a secret given as exactly one reference reads
	// it.
	verbatim bool
	// keepFiles leaves `${file:}` references as they are, for a type that
	// decodes itself, whose secrets read them.
	keepFiles bool
	// refs collects the references the current value was read through.
	refs []string
}

// asGiven is r for values from the environment and flags.
func (r *resolver) asGiven() *resolver {
	return &resolver{lookup: r.lookup, schemes: r.schemes, verbatim: true}
}

// expand resolves the references in s, a string from the configuration file
// going into a field that is not a secret.
func (r *resolver) expand(s string) (string, error) {
	if r.verbatim || !strings.Contains(s, "$") {
		return s, nil
	}

	b := strings.Builder{}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '$' || i+1 == len(s) {
			b.WriteByte(c)
			continue
		}
		switch s[i+1] {
		case '$':
			b.WriteByte('$')
			i++
		case '{':
			end := strings.IndexByte(s[i:], '}')
			if end < 0 {
				return "", fmt.Errorf("%q: unterminated reference", s[i:])
			}
			ref := s[i : i+end+1]
			v, err := r.expandRef(ref)
			if err != nil {
				return "", err
			}
			b.WriteString(v)
			i += end
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// parseRef splits "${scheme:rest}".
func parseRef(ref string) (scheme string, rest string, err error) {
	body, ok := strings.CutPrefix(ref, "${")
	if ok {
		body, ok = strings.CutSuffix(body, "}")
	}
	if !ok {
		return "", "", fmt.Errorf("%q: not a reference", ref)
	}
	scheme, rest, ok = strings.Cut(body, ":")
	if !ok || scheme == "" {
		return "", "", fmt.Errorf("%q: want ${scheme:...}", ref)
	}
	if strings.Contains(rest, "${") {
		// `${env:A:-${env:B}}`: the first "}" would end the outer one.
		return "", "", fmt.Errorf("%q: a reference cannot hold another", ref)
	}
	return scheme, rest, nil
}

func (r *resolver) expandRef(ref string) (string, error) {
	scheme, rest, err := parseRef(ref)
	if err != nil {
		return "", err
	}
	switch scheme {
	case "env":
		v, err := r.env(rest)
		if err != nil {
			return "", err
		}
		r.refs = append(r.refs, ref)
		return v, nil
	case "file":
		if r.keepFiles {
			r.refs = append(r.refs, ref)
			return ref, nil
		}
		return "", fmt.Errorf("%s: a file is only read into a secret field (cfg.Secret), which re-reads it when it changes", ref)
	}
	v, err := r.scheme(scheme, rest)
	if err != nil {
		return "", fmt.Errorf("%s: %w", ref, err)
	}
	r.refs = append(r.refs, ref)
	return string(v), nil
}

// env resolves the body of `${env:NAME}` or `${env:NAME:-default}`.
func (r *resolver) env(body string) (string, error) {
	name, def, hasDef := strings.Cut(body, ":-")
	if !validEnvName(name) {
		return "", fmt.Errorf("${env:%s}: %q is not a variable name", body, name)
	}
	if v, ok := r.lookup(name); ok {
		return v, nil
	}
	if hasDef {
		return def, nil
	}
	return "", fmt.Errorf("${env:%s}: %s is not set", body, name)
}

func validEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

func (r *resolver) scheme(name string, ref string) ([]byte, error) {
	f, ok := r.schemes[name]
	if !ok {
		return nil, fmt.Errorf("unknown scheme %q", name)
	}
	return f(ref)
}

var durationType = reflect.TypeFor[time.Duration]()

// setText reads s into v, as from an environment variable or a flag: lists and
// maps are comma separated or YAML flow syntax. References are not expanded
// except by secret fields.
func setText(v reflect.Value, s string, r *resolver) error {
	if v.Kind() == reflect.Pointer {
		p := reflect.New(v.Type().Elem())
		err := setText(p.Elem(), s, r)
		if err != nil && !isPending(err) {
			return err
		}
		v.Set(p)
		return err
	}
	if sf, ok := v.Addr().Interface().(secretField); ok {
		return sf.setSecret(s, r)
	}
	if readsItself(v.Type()) {
		// Read whole, into a new value, as from the file.
		fresh := reflect.New(v.Type())
		var err error
		if tu, ok := fresh.Interface().(encoding.TextUnmarshaler); ok {
			err = tu.UnmarshalText([]byte(s))
		} else {
			// A YAML unmarshaler: read the text as a YAML value.
			err = yaml.Unmarshal([]byte(s), fresh.Interface())
		}
		if err != nil {
			return err
		}
		v.Set(fresh.Elem())
		return nil
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
			return setFlow(v, s, r)
		}
		if v.Kind() == reflect.Map {
			return setPlainMap(v, s, r)
		}
		return setPlainList(v, s, r)
	}
	return setScalar(v, s)
}

// setScalar reads s into a value of a basic kind.
func setScalar(v reflect.Value, s string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return fmt.Errorf("%q is not a boolean", s)
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.Type() == durationType {
			d, err := time.ParseDuration(s)
			if err != nil {
				return fmt.Errorf("%q is not a duration", s)
			}
			v.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(s, intBase(s), v.Type().Bits())
		if err != nil || strings.Contains(s, "_") {
			return fmt.Errorf("%q is not an integer of %d bits", s, v.Type().Bits())
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(s, intBase(s), v.Type().Bits())
		if err != nil || strings.Contains(s, "_") {
			return fmt.Errorf("%q is not an unsigned integer of %d bits", s, v.Type().Bits())
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := parseFloat(s, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("%q is not a number", s)
		}
		v.SetFloat(n)
	case reflect.Interface:
		if v.NumMethod() != 0 {
			return fmt.Errorf("cannot read text into %s", v.Type())
		}
		v.Set(reflect.ValueOf(s))
	default:
		return fmt.Errorf("cannot read text into %s", v.Type())
	}
	return nil
}

// intBase is the base an integer is written in, as YAML 1.2 reads it: decimal,
// or hexadecimal, octal or binary with a 0x, 0o or 0b prefix. A leading zero
// does not make a number octal: 010 is ten.
func intBase(s string) int {
	t := strings.TrimLeft(s, "+-")
	if len(t) > 2 && t[0] == '0' && strings.ContainsRune("xXoObB", rune(t[1])) {
		return 0
	}
	return 10
}

// parseFloat is strconv.ParseFloat that also reads YAML's .inf and .nan.
func parseFloat(s string, bits int) (float64, error) {
	switch strings.ToLower(s) {
	case ".inf", "+.inf":
		return math.Inf(1), nil
	case "-.inf":
		return math.Inf(-1), nil
	case ".nan":
		return math.NaN(), nil
	}
	return strconv.ParseFloat(s, bits)
}

// setFlow reads YAML flow syntax, e.g. `[a, "b,c"]` or `{k: v}`. Every error
// is returned, each at the item it is about.
func setFlow(v reflect.Value, s string, r *resolver) error {
	f, err := parser.ParseBytes([]byte(s), 0)
	if err != nil {
		return fmt.Errorf("%q: %w", s, err)
	}
	d := &decoder{r: r}
	body, err := d.body(f)
	if err != nil {
		return err
	}
	vs := []error{}
	for _, err := range d.decode(body, v, "", false) {
		var fe *FieldError
		if errors.As(err, &fe) {
			// The value's own position in one line says little.
			err = fe.Err
			if fe.Origin.Key != "" {
				err = within(fe.Origin.Key, err)
			}
		}
		vs = append(vs, err)
	}
	return joinItems(vs)
}

// setPlainList reads a comma separated list. An item that fails is reported,
// and the others are read; an item whose secret file is not there yet is kept.
func setPlainList(v reflect.Value, s string, r *resolver) error {
	items := splitEscaped(s, ',')
	l := reflect.New(v.Type()).Elem()
	if v.Kind() == reflect.Array {
		if len(items) != v.Len() {
			return fmt.Errorf("want %d values, got %d", v.Len(), len(items))
		}
	} else {
		l = reflect.MakeSlice(v.Type(), len(items), len(items))
	}

	es := []error{}
	for i, item := range items {
		if err := setText(l.Index(i), unescape(item), r); err != nil {
			es = append(es, within(fmt.Sprintf("[%d]", i), err))
		}
	}
	if err := joinItems(es); err != nil && !isPending(err) {
		return err
	}
	v.Set(l)
	return joinItems(es)
}

// setPlainMap reads a comma separated list of key=value. As setPlainList.
func setPlainMap(v reflect.Value, s string, r *resolver) error {
	m := reflect.MakeMap(v.Type())
	es := []error{}
	for _, item := range splitEscaped(s, ',') {
		kv := splitEscaped(item, '=')
		if len(kv) < 2 {
			es = append(es, fmt.Errorf("%q: want key=value", unescape(item)))
			continue
		}
		// The value is everything after the first "=".
		key, val := unescape(kv[0]), unescape(item[len(kv[0])+1:])

		k := reflect.New(v.Type().Key()).Elem()
		if err := setText(k, key, r); err != nil {
			es = append(es, within(fmt.Sprintf("key %q", key), err))
			continue
		}
		e := reflect.New(v.Type().Elem()).Elem()
		if err := setText(e, val, r); err != nil {
			es = append(es, within(key, err))
			if !isPending(err) {
				continue
			}
		}
		m.SetMapIndex(k, e)
	}
	if err := joinItems(es); err != nil && !isPending(err) {
		return err
	}
	v.Set(m)
	return joinItems(es)
}

// splitEscaped splits s at every sep not escaped by a backslash. The parts keep
// their escapes.
func splitEscaped(s string, sep byte) []string {
	vs := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case sep:
			vs = append(vs, s[start:i])
			start = i + 1
		}
	}
	return append(vs, s[start:])
}

// unescape drops the backslash of every escaped character.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	b := strings.Builder{}
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

var errEmpty = errors.New("empty")
