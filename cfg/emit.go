package cfg

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// flowOf writes v, plain values as anyOf gives them, in YAML flow syntax:
// mappings in order and with keys of their own types, which goccy/go-yaml's
// encoder does not do for a MapSlice.
func flowOf(v any) string {
	b := &strings.Builder{}
	writeFlow(b, v)
	return b.String()
}

func writeFlow(b *strings.Builder, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case yaml.MapSlice:
		b.WriteByte('{')
		for i, it := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeFlow(b, it.Key)
			b.WriteString(": ")
			writeFlow(b, it.Value)
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteString(", ")
			}
			writeFlow(b, e)
		}
		b.WriteByte(']')
	case string:
		b.WriteString(quoted(v))
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case float64:
		b.WriteString(formatFloat(v))
	default:
		b.WriteString(quoted(fmt.Sprint(v)))
	}
}

// quoted is s as a YAML double-quoted scalar.
func quoted(s string) string {
	b := strings.Builder{}
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f || r == 0x85 || r == 0x2028 || r == 0x2029 || r == 0xFEFF {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// formatFloat writes f so that it reads back as a float: 1 is "1.0".
func formatFloat(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return ".inf"
	case math.IsInf(f, -1):
		return "-.inf"
	case math.IsNaN(f):
		return ".nan"
	}
	s := strconv.FormatFloat(f, 'g', -1, 64)
	if !strings.ContainsAny(s, ".e") {
		s += ".0"
	}
	return s
}
