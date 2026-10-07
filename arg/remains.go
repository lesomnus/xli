package arg

import (
	"fmt"
)

// Remains is the rest of the command line after "--", as it is: the
// arguments a command passes on to another program.
type Remains = Base[[]string, RemainsParser]

// RemainsParser takes "--" and everything after it, which it returns without
// the "--".
type RemainsParser struct{}

func (RemainsParser) Parse(rest []string) ([]string, int, error) {
	if rest[0] != "--" {
		return nil, 0, fmt.Errorf(`it must start with "--"`)
	}
	return rest[1:], len(rest), nil
}

func (RemainsParser) String() string {
	return "--"
}
