package mode

import (
	"context"
)

type Mode int

const (
	Unspecified Mode = 0b00_0

	Pass Mode = 0b000_1 // There are more commands to be executed.
	Help Mode = 0b001_0 // Command is executed to print help message.
	Tab  Mode = 0b010_0 // Command is executed to get completions.
	Run  Mode = 0b100_0 // Command is executed to do something.

	Kind Mode = 0b111_0
)

func (m Mode) Is(v Mode) bool {
	return m&v == v
}

func (m Mode) NoPass() Mode {
	return m & ^Pass
}

type ctxKey struct{}

func From(ctx context.Context) Mode {
	v, ok := ctx.Value(ctxKey{}).(Mode)
	if !ok {
		return Unspecified
	}

	return v
}

func Into(ctx context.Context, v Mode) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}
