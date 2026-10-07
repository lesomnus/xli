// Package xmd is what the frm package knows of a command, so that frm need
// not import xli, which imports it.
package xmd

import (
	"github.com/lesomnus/xli/arg"
	"github.com/lesomnus/xli/flg"
)

// Command is a command as a frame holds it: an *xli.Command.
type Command interface {
	GetName() string
	GetFlags() flg.Flags
	GetArgs() arg.Args
}
