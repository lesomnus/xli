package xli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/lesomnus/xli/internal/lex"
	"github.com/lesomnus/xli/internal/suggest"
)

// What can be wrong with a command line. Run returns them wrapped in a
// [UsageError] that names the command, and a mistake about one token also in
// a [FlagError] or an [ArgError] that names it, so they are matched with
// errors.Is.
var (
	// ErrUnknownFlag is a flag the command does not declare. A flag of a
	// parent or a subcommand is unknown to this one: positions are strict.
	ErrUnknownFlag = errors.New("unknown flag")
	// ErrNoFlagValue is a flag that takes a value given none.
	ErrNoFlagValue = errors.New("no value is given")
	// ErrFlagRequired is a flag declared Required that was not given.
	ErrFlagRequired = errors.New("required flag not set")
	// ErrFlagConflict is two flags of one of the command's Exclusive groups
	// given together.
	ErrFlagConflict = errors.New("flags cannot be used together")
	// ErrFlagAfterArg is a flag given after an argument of the same command.
	ErrFlagAfterArg = errors.New("flag must come before arguments")
	// ErrUnknownCmd is a word that is neither an argument nor the name of a
	// subcommand.
	ErrUnknownCmd = errors.New("unknown subcommand")
	// ErrTooManyArgs is an argument more than the command declares.
	ErrTooManyArgs = errors.New("too many arguments")
	// ErrNeedArgs is a required argument not given.
	ErrNeedArgs = errors.New("required argument not given")
	// ErrNeedCmd is a command run without the subcommand it requires; see
	// [RequireSubcommand].
	ErrNeedCmd = errors.New("subcommand is required")
)

// FlagError is a mistake about one flag, saying it as it was written, with
// the flags it may have been meant to be.
type FlagError struct {
	flag lex.Flag
	err  error

	// suggestions are likely intended flags for an unknown one.
	suggestions []string
}

// Error is the flag as it was written, what is wrong with it, and what it may
// have been meant to be.
func (e *FlagError) Error() string {
	return fmt.Sprintf("%s: %s%s", e.flag.WithoutArg().Raw(), e.err.Error(), suggest.Hint(e.suggestions))
}

// Unwrap is what is wrong with the flag, such as [ErrUnknownFlag].
func (e *FlagError) Unwrap() error {
	return e.err
}

// ArgError is a mistake about one word of the command line that is not a flag:
// an argument, or a subcommand that is not there, with the ones it may have
// been meant to be.
type ArgError struct {
	arg lex.Arg
	err error

	// suggestions are likely intended subcommands for an unknown one.
	suggestions []string
}

// Error is the word as it was written, what is wrong with it, and what it may
// have been meant to be.
func (e *ArgError) Error() string {
	return fmt.Sprintf("%s: %s%s", e.arg.Raw(), e.err.Error(), suggest.Hint(e.suggestions))
}

// Unwrap is what is wrong with the word, such as [ErrUnknownCmd].
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
// Errors returned by flag handlers are treated as invalid flag values and
// reported as UsageErrors too. Errors returned by command handlers are passed
// through as is.
type UsageError struct {
	// Cmd is the command whose flags, arguments, or subcommands were misused.
	Cmd *Command
	Err error
}

// Error is the path of the command and what is wrong.
func (e *UsageError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path(), e.Err.Error())
}

// Unwrap is what is wrong.
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
