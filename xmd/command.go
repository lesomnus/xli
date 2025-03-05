package xmd

import (
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
)

type Command interface {
	GetName() string
	GetFlags() flg.Flags
	GetArgs() arg.Args
}
