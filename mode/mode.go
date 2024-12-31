package mode

import (
	"context"
	"slices"
)

type Mode int

const (
	Unspecified Mode = 0b00_0

	Pass = 0b000_1 // There are more commands to be executed.
	Help = 0b001_0 // Command is executed to print help message.
	Tab  = 0b010_0 // Command is executed to get completions.
	Run  = 0b100_0 // Command is executed to do something.

	Kind = 0b111_0
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

func Resolve(args []string) Mode {
	if slices.ContainsFunc(args, func(v string) bool {
		return v == "--help" || v == "-h"
	}) {
		return Help
	}

	return Run
}
