// Package mode is why a command line is run: to run a command, to print its
// help, or to complete it in a shell. Handlers see the mode in their context
// and run in the ones they are for; see xli.OnRun and the other On functions.
package mode

import (
	"context"
)

// Mode is a bit set: one kind -- Run, Help or Tab -- and Pass if the command
// whose handler sees it is on the way to another.
type Mode int

const (
	Unspecified Mode = 0b00_0

	Pass Mode = 0b000_1 // There are more commands to be executed.
	Help Mode = 0b001_0 // Command is executed to print help message.
	Tab  Mode = 0b010_0 // Command is executed to get completions.
	Run  Mode = 0b100_0 // Command is executed to do something.

	Kind Mode = 0b111_0
)

// Is reports whether m has every bit of v set.
func (m Mode) Is(v Mode) bool {
	return m&v == v
}

// NoPass is m without Pass.
func (m Mode) NoPass() Mode {
	return m & ^Pass
}

type ctxKey struct{}

// From is the mode ctx carries, or Unspecified.
func From(ctx context.Context) Mode {
	v, ok := ctx.Value(ctxKey{}).(Mode)
	if !ok {
		return Unspecified
	}

	return v
}

// Into is ctx carrying v, for [From].
func Into(ctx context.Context, v Mode) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}
