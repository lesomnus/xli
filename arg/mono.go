package arg

import (
	"github.com/lesomnus/xli/flg"
)

type MonoParser[T any, P flg.Parser[T]] struct {
	P P
}

type Mono[T any, P flg.Parser[T]] = Base[T, MonoParser[T, P]]

func (p MonoParser[T, P]) Parse(rest []string) (T, int, error) {
	v, err := p.P.Parse(rest[0])
	if err != nil {
		return v, 0, err
	}

	return v, 1, nil
}

func (p MonoParser[T, P]) String() string {
	return p.P.String()
}
