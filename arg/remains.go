package arg

import (
	"fmt"
)

type Remains = Base[[]string, RemainsParser]

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
