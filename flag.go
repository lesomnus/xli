package xli

import (
	"context"
)

type FlagInfo struct {
	Name    string
	Aliases []rune

	Brief string
	Synop string
	Usage Stringer
}

type Flag interface {
	Info() *FlagInfo
	Handle(ctx context.Context, cmd *Command, v string) (context.Context, error)

	Default() (string, bool)
}

type Flags []Flag

func (fs Flags) Get(name string) Flag {
	for _, f := range fs {
		if f.Info().Name == name {
			return f
		}
	}

	return nil
}
