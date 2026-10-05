package flg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lesomnus/xli/internal/suggest"
	"github.com/lesomnus/xli/tab"
)

// Choice is a string flag restricted to a fixed set of values:
//
//	&flg.Choice{Name: "format", Parser: flg.ChoiceParser{"json", "yaml"}}
//
// Any other value is rejected, the choices are shown as the flag's type in
// help ("json|yaml"), and they are offered as shell completion candidates.
type Choice = Base[string, ChoiceParser]

// ChoiceParser accepts only the listed values.
type ChoiceParser []string

func (p ChoiceParser) Parse(s string) (string, error) {
	return parseChoice(p, s)
}

func (ChoiceParser) ToString(v string) string {
	return v
}

func (p ChoiceParser) String() string {
	return strings.Join(p, "|")
}

// Complete offers the choices as completion candidates.
func (p ChoiceParser) Complete(t tab.Tab) {
	for _, v := range p {
		t.Value(v)
	}
}

func parseChoice(choices []string, s string) (string, error) {
	if slices.Contains(choices, s) {
		return s, nil
	}

	qs := make([]string, len(choices))
	for i, v := range choices {
		qs[i] = fmt.Sprintf("%q", v)
	}
	return "", fmt.Errorf("invalid value %q: must be one of %s%s", s, strings.Join(qs, ", "), suggest.Hint(suggest.Of(s, choices)))
}

// complete lets a parser that knows its candidates (e.g. ChoiceParser) offer
// them during completion.
func complete(t tab.Tab, p any) {
	if t == nil {
		return
	}
	if c, ok := p.(interface{ Complete(t tab.Tab) }); ok {
		c.Complete(t)
	}
}
