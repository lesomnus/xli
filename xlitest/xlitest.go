// Package xlitest provides helpers for testing commands built with xli.
//
// Testing a command means running it and observing what it did: what it wrote,
// what error it returned, and — for shell completion — which candidates it
// offered. Doing this by hand means wiring three IO fields, feeding a stdin
// that will not block on the real terminal, and, for completion, knowing the
// undocumented argv protocol and the wire format the zsh sink emits.
//
// xlitest hides all of that:
//
//   - [Run] wires in-memory IO, runs the command, and returns captured
//     stdout/stderr and the error.
//   - [Complete] drives the completion path from a natural command line and
//     decodes the output into structured [Candidate]s.
//
// A command keeps the values parsed into its flags and arguments, so running
// the same *xli.Command twice lets the first run's state leak into the second.
// Build a fresh command per run with a factory:
//
//	newCmd := func() *xli.Command { return &xli.Command{ /* ... */ } }
//	got := xlitest.Run(t, newCmd(), "--verbose")
package xlitest

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/internal/comp"
)

// Result captures the outcome of running a command.
type Result struct {
	// Stdout is everything the command wrote to its Writer.
	Stdout string
	// Stderr is everything the command wrote to its ErrWriter.
	Stderr string
	// Err is the error returned by Command.Run, if any.
	Err error
}

// Harness configures a command run. Set Cmd; Ctx and Stdin are optional.
//
// A Harness runs a single *xli.Command, which it mutates (it sets the command's
// IO fields and the run fills in parsed flag/argument values). Reusing one
// Harness for several runs therefore reuses one command; when the runs must be
// independent, build a new Harness around a fresh command each time.
type Harness struct {
	// Cmd is the command under test.
	Cmd *xli.Command
	// Ctx is the context passed to Command.Run. A nil Ctx defaults to
	// context.Background().
	Ctx context.Context
	// Stdin is fed to the command as standard input. The default (empty) reads
	// as immediate EOF; either way the command never touches the real os.Stdin.
	Stdin string
}

// run wires the command's IO to in-memory buffers and executes it. The command
// never reads the real os.Stdin or writes the real os.Stdout/os.Stderr.
func (h Harness) run(t testing.TB, args []string) (stdout, stderr *bytes.Buffer, err error) {
	t.Helper()
	if h.Cmd == nil {
		t.Fatal("xlitest: Harness.Cmd is nil")
		return nil, nil, nil
	}

	ctx := h.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	h.Cmd.ReadCloser = io.NopCloser(strings.NewReader(h.Stdin))
	h.Cmd.Writer = stdout
	h.Cmd.ErrWriter = stderr

	err = h.Cmd.Run(ctx, args)
	return stdout, stderr, err
}

// Run executes the command with args and returns its captured output.
func (h Harness) Run(t testing.TB, args ...string) Result {
	t.Helper()
	stdout, stderr, err := h.run(t, args)
	return Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    err,
	}
}

// Run executes cmd with args and returns its captured output. It is shorthand
// for Harness{Cmd: cmd}.Run. Pass a freshly built command (see the package
// docs) when the same command is run more than once.
func Run(t testing.TB, cmd *xli.Command, args ...string) Result {
	t.Helper()
	return Harness{Cmd: cmd}.Run(t, args...)
}

// Candidate is one completion suggestion.
type Candidate struct {
	// Group is the heading the candidate is shown under; empty when ungrouped.
	Group string
	// Value is the text inserted when the candidate is selected.
	Value string
	// Desc is the description shown beside the value; empty when none.
	Desc string
}

// Completions is the decoded result of a completion run.
type Completions struct {
	// Candidates are the value suggestions, in the order they were emitted.
	Candidates []Candidate
	// FilePatterns holds the non-empty globs for which file completion was
	// requested (e.g. "*.go"). A bare file request adds no pattern but still
	// sets WantFiles.
	FilePatterns []string
	// WantFiles reports whether file completion was requested at all.
	WantFiles bool
	// WantDirs reports whether directory-only completion was requested.
	WantDirs bool
	// Err is the error returned by the completion run, if any. Completion is
	// expected to succeed and emit nothing on trouble, so a non-nil Err usually
	// means the line was malformed (for example an unknown flag earlier in it).
	Err error
	// Raw is the undecoded sink output, for when a test needs the wire format.
	Raw string
}

// Values returns the Value of each candidate, in order.
func (c Completions) Values() []string {
	vs := make([]string, len(c.Candidates))
	for i, cand := range c.Candidates {
		vs[i] = cand.Value
	}
	return vs
}

// Has reports whether any candidate has the given value.
func (c Completions) Has(value string) bool {
	_, ok := c.Get(value)
	return ok
}

// Get returns the first candidate with the given value.
func (c Completions) Get(value string) (Candidate, bool) {
	for _, cand := range c.Candidates {
		if cand.Value == value {
			return cand, true
		}
	}
	return Candidate{}, false
}

// Complete drives the command's shell-completion path as if the user pressed
// <Tab> after typing line, with the cursor at the end of line. line is the
// tokens that follow the program name.
//
// A trailing space means the cursor sits at a fresh, empty word, so the next
// token is being started; without it, the final token is the word being
// completed. For example, given a "remote" subcommand with "add"/"remove":
//
//	Complete(t, cmd, "remote ")   // offers: add, remove
//	Complete(t, cmd, "remote a")  // offers: add
//	Complete(t, cmd, "--")        // offers: flag names
//	Complete(t, cmd, "--tag=")    // offers: values for --tag
//
// line is split on whitespace; shell quoting and escapes are not interpreted.
// Pass a freshly built command (see the package docs) when the same command is
// completed more than once.
func Complete(t testing.TB, cmd *xli.Command, line string) Completions {
	t.Helper()
	return Harness{Cmd: cmd}.Complete(t, line)
}

// Complete drives the command's completion path for line; see the package-level
// Complete for the semantics of line.
func (h Harness) Complete(t testing.TB, line string) Completions {
	t.Helper()

	args, curr, buff := splitLine(line)
	full := make([]string, 0, len(args)+3)
	full = append(full, args...)
	full = append(full, comp.Tag("zsh"), curr, buff)

	stdout, _, err := h.run(t, full)
	c := decodeZsh(stdout.String())
	c.Err = err
	return c
}

// splitLine turns a typed command line into the (args, curr, buff) triple the
// completion path expects. curr is the word under the cursor and buff is the
// portion of it to the left of the cursor; with the cursor at the end of the
// line the two are equal. A line that is empty or ends in whitespace places the
// cursor at a fresh, empty word.
func splitLine(line string) (args []string, curr, buff string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return fields, "", ""
	}
	if last := line[len(line)-1]; last == ' ' || last == '\t' {
		return fields, "", ""
	}
	curr = fields[len(fields)-1]
	return fields, curr, curr
}

// decodeZsh parses the lines emitted by xli's zsh completion sink. Each line is
// one of:
//
//	v<sep><group><sep><entry>   a candidate ("value" or "value:desc")
//	f<sep><pattern>             a file-completion request (pattern may be empty)
//	d<sep>                      a directory-completion request
func decodeZsh(raw string) Completions {
	c := Completions{Raw: raw}
	for _, line := range strings.Split(raw, "\n") {
		kind, rest, ok := strings.Cut(line, comp.Sep)
		if !ok {
			continue
		}
		switch kind {
		case "v":
			group, entry, ok := strings.Cut(rest, comp.Sep)
			if !ok || entry == "" {
				continue
			}
			value, desc, _ := strings.Cut(entry, ":")
			c.Candidates = append(c.Candidates, Candidate{
				Group: group,
				Value: value,
				Desc:  desc,
			})
		case "f":
			c.WantFiles = true
			if rest != "" {
				c.FilePatterns = append(c.FilePatterns, rest)
			}
		case "d":
			c.WantDirs = true
		}
	}
	return c
}
