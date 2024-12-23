package arg

import (
	"context"
)

type String = Base[string, StringParser]

type StringParser struct{}

func (StringParser) Parse(ctx context.Context, prev []string, rest []string) (string, int, error) {
	if len(rest) == 0 {
		return "", 0, nil
	}
	return rest[0], 1, nil
}

func (StringParser) String() string {
	return "string"
}
