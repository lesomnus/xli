package arg

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lesomnus/xli/internal/suggest"
	"github.com/lesomnus/xli/tab"
)

// Choice is a string argument restricted to a fixed set of values:
//
//	&arg.Choice{Name: "SHELL", Parser: arg.ChoiceParser{"bash", "zsh", "fish"}}
//
// Any other value is rejected and the choices are offered as shell completion
// candidates.
type Choice = Base[string, ChoiceParser]

// ChoiceParser accepts only the listed values.
type ChoiceParser []string

func (p ChoiceParser) Parse(rest []string) (string, int, error) {
	s := rest[0]
	if slices.Contains(p, s) {
		return s, 1, nil
	}

	qs := make([]string, len(p))
	for i, v := range p {
		qs[i] = fmt.Sprintf("%q", v)
	}
	return "", 1, fmt.Errorf("must be one of %s%s", strings.Join(qs, ", "), suggest.Hint(suggest.Of(s, p)))
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
