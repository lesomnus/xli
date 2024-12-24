package arg

import (
	"context"
)

type String = Base[string, StringParser]

type StringParser struct{}

func (StringParser) Parse(ctx context.Context, rest []string) (string, int, error) {
	return rest[0], 1, nil
}

func (StringParser) String() string {
	return "string"
}
