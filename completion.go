package xli

import (
	"context"
	"embed"
	"strings"
)

const completion_tag_prefix = "$$xli_completion_"

//go:embed completions
var completions embed.FS

func NewCmdCompletion() *Command {
	return &Command{
		Name: "completion",
		Commands: Commands{
			newCmdZshCompletion(),
		},
	}
}

func newCmdZshCompletion() *Command {
	return &Command{
		Name: "zsh",
		Handler: OnRun(func(ctx context.Context, cmd *Command, next Next) error {
			b, err := completions.ReadFile("completions/zsh")
			if err != nil {
				panic(err)
			}

			c := cmd.Parent().Parent()
			if _, err := cmd.Printf(string(b), c.Name); err != nil {
				panic(err)
			}

			return next(ctx)
		}),
	}
}

// Normalize arguments for completion.
// Last item of given `args` maybe modified.
// `curr` is the word where the cursor is on so it can empty string even if `args` is not empty.
// `buff` is the string on the left of the cursor which size is less then `curr`.
//
// Normalized `args` will be one of following form:
//
//	// Suggest subcommands if the command does not accepts arg.
//	// or values for the arg if the command accepts arg.
//	[]
//	[..., "arg"]
//	[..., "cmd"]
//	[..., "--flag=val"]
//	[..., "--flag", "val"]
//
//	// Suggest flags.
//	[..., "--"]
//
//	// Suggest values for the flag.
//	[..., "--flag"]
//	[..., "--flag="]
//
// `curr` is empty if the cursor is in the next arg position like:
//
//	$ foo bar
//	          ^
func NormalizeCompletionArgs(args []string, curr string, buff string) []string {
	if len(args) == 0 {
		return args
	}
	if curr == "" {
		return args
	}
	for i := 0; i < len(curr); i++ {
		if strings.HasPrefix(curr, buff[i:]) {
			buff = buff[i:]
			break
		}
	}
	if buff == "" || buff[0] != '-' {
		args = args[:len(args)-1]
	} else {
		if i := strings.IndexRune(buff, '='); i > 0 {
			// /-.+=.+/ => /-.+=/
			buff = buff[:i+1]
		} else {
			// /-.+/ => "--"
			buff = "--"
		}
		args[len(args)-1] = buff
	}

	return args
}
