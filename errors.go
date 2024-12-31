package xli

import (
	"errors"
	"fmt"

	"github.com/lesomnus/xli/lex"
)

var (
	ErrUnknownFlag = errors.New("unknown flag")
	ErrNoFlagValue = errors.New("no value is given")
	ErrUnknownCmd  = errors.New("unknown subcommand")
	ErrTooManyArgs = errors.New("too many arguments")
)

type FlagError struct {
	flag lex.Flag
	err  error
}

func (e *FlagError) Error() string {
	return fmt.Sprintf("%s: %s", e.flag.WithoutArg().Raw(), e.err.Error())
}

func (e *FlagError) Unwrap() error {
	return e.err
}

type ArgError struct {
	arg lex.Arg
	err error
}

func (e *ArgError) Error() string {
	return fmt.Sprintf("%s: %s", e.arg.Raw(), e.err.Error())
}

func (e *ArgError) Unwrap() error {
	return e.err
}
