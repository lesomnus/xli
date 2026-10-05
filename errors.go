package xli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lesomnus/xli/lex"
)

var (
	ErrUnknownFlag  = errors.New("unknown flag")
	ErrNoFlagValue  = errors.New("no value is given")
	ErrFlagRequired = errors.New("required flag not set")
	ErrFlagAfterArg = errors.New("flag must come before arguments")
	ErrUnknownCmd   = errors.New("unknown subcommand")
	ErrTooManyArgs  = errors.New("too many arguments")
	ErrNeedArgs     = errors.New("required argument not given")
	ErrNeedCmd      = errors.New("subcommand is required")
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

// UsageError reports that the command line does not match what a command
// declares: an unknown flag or subcommand, a missing or malformed value, a
// missing required flag or argument, and so on. It identifies the command the
// mistake was made on, so callers can point the user at that command's help:
//
//	var ue *xli.UsageError
//	if errors.As(err, &ue) {
//		ue.Cmd.PrintHelp(os.Stderr)
//	}
//
// Errors returned by handlers are passed through as is and are not wrapped.
type UsageError struct {
	// Cmd is the command whose flags, arguments, or subcommands were misused.
	Cmd *Command
	Err error
}

func (e *UsageError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path(), e.Err.Error())
}

func (e *UsageError) Unwrap() error {
	return e.Err
}

// Path returns the space-separated command path from the root to Cmd, e.g.
// "app deploy".
func (e *UsageError) Path() string {
	tree := e.Cmd.Tree()
	names := make([]string, len(tree))
	for i, c := range tree {
		names[i] = c.Name
	}
	return strings.Join(names, " ")
}
