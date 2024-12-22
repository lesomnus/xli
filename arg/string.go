package arg

import (
	"context"

	"github.com/lesomnus/xli"
)

type String = Arg[string, StringParser]

type StringParser struct{}

func (p StringParser) Prase(ctx context.Context, cmd *xli.Command, pre []string, rest []string) (string, int, error) {
	if len(rest) == 0 {
		return "", 0, nil
	}
	return rest[0], 1, nil
}
