// Package comp holds the wire protocol shared between xli's completion
// machinery and its shell integration: the argv tag that switches a normal run
// into the completion path, and the field separator used on each emitted line.
//
// These values are the single source of truth for the Go side (the root xli
// package, tab, and xlitest). The generated shell scripts under completions/
// hardcode the same values and must be kept in sync by hand.
package comp

// TagPrefix is prepended to a shell name to form the completion trigger tag.
// The generated shell script passes that tag as the third-to-last argument, and
// Command.Run switches into the completion path when it sees this prefix.
const TagPrefix = "$$xli_completion_"

// Sep separates fields on each emitted completion line. It is a non-printing
// byte that survives shell command substitution (unlike NUL).
const Sep = "\x1f"

// Tag returns the completion trigger tag for the named shell, e.g.
// Tag("zsh") == "$$xli_completion_zsh".
func Tag(shell string) string {
	return TagPrefix + shell
}
