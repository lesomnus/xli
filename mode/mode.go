package mode

import (
	"context"
	"slices"
	"strings"
)

type Mode int

const (
	Unspecified Mode = 0b00_00

	Pass = 0b00_11 // There are more commands to be executed.
	Help = 0b00_01 // Command is executed to print help message.
	Tap  = 0b01_01 // Command is executed to get completions.
	Run  = 0b10_01 // Command is executed to do something.
)

func (m Mode) Is(v Mode) bool {
	return m&v == v
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
		return strings.HasPrefix(v, "--help") || strings.HasPrefix(v, "-h")
	}) {
		return Help
	}

	return Run
}
