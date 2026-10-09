package xli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
)

// agentEnvs are environment variables an AI coding agent sets for the commands
// it runs. AI_AGENT is the one being agreed on (agentsmd/agents.md#136); the
// rest are the agents' own, for those that do not set it yet. CURSOR_TRACE_ID
// is not one of them: Cursor's terminal sets it for a person too.
var agentEnvs = []string{
	"AI_AGENT",
	"CLAUDECODE",
	"CODEX_CI", "CODEX_SANDBOX", "CODEX_THREAD_ID",
	"CURSOR_AGENT",
	"GEMINI_CLI",
}

// agentEnv decides instead of agentEnvs, as strconv.ParseBool reads it:
// XLI_AGENT=0 says no agent runs the command, and XLI_AGENT=1 says one does.
const agentEnv = "XLI_AGENT"

// byAgent reports whether the environment says an AI agent runs the command.
func byAgent() bool {
	if v, err := strconv.ParseBool(os.Getenv(agentEnv)); err == nil {
		return v
	}
	for _, k := range agentEnvs {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}

// agentHelpMax is the most help --help prints at once for an agent: about four
// thousand tokens. It is all of the help of most command lines, and well
// within the output an agent keeps of a command; a tree of hundreds of
// commands is neither.
const agentHelpMax = 16 << 10

// printAgentHelp is --help for an AI agent. A person reads the help of one
// command and asks for the next; an agent would ask one level at a time down
// a tree it could have been given whole. So it is the help of c and of every
// command below it, as --help-all prints it, if that is no more than
// agentHelpMax.
//
// If it is more, it is the help of c as a person gets it and a map of every
// command below it (see commandMap), which a tree of hundreds of commands
// fits in a few lines of, since most of them are the same few verbs on a
// list of resources. The agent asks the one it wants for its help. If even
// the map does not fit beside the help of c, it says only how many commands
// there are. Either way it says how long --help-all is, so that it is not
// asked for blindly; the help of c itself is never left out, however long.
func (c *Command) printAgentHelp(w io.Writer) error {
	n := countBelow(c)
	if n == 0 {
		return c.PrintHelp(w)
	}

	b := &capWriter{max: agentHelpMax}
	if err := c.printHelpAll(b); err == nil {
		_, err = w.Write(b.buf.Bytes())
		return err
	} else if !errors.Is(err, errCapped) {
		return err
	}

	all := &countWriter{}
	if err := c.printHelpAll(all); err != nil {
		return err
	}
	own := &bytes.Buffer{}
	if err := c.PrintHelp(own); err != nil {
		return err
	}

	m := &strings.Builder{}
	if n == 1 {
		m.WriteString("\nThe command below:\n")
	} else {
		fmt.Fprintf(m, "\nThe %d commands below ({a,b} is a or b):\n", n)
	}
	for _, l := range commandMap(c) {
		m.WriteString("    " + l + "\n")
	}

	size := kb(all.n)
	switch fits := own.Len()+m.Len() <= agentHelpMax; {
	case fits && n == 1:
		own.WriteString(m.String())
		fmt.Fprintf(own, "\nIts help is more than an AI agent's --help prints at once. --help on it prints\nits help; --help-all prints all of it (%s).\n", size)
	case fits:
		own.WriteString(m.String())
		fmt.Fprintf(own, "\nTheir help is more than an AI agent's --help prints at once. --help on one of\nthem prints its help with the commands below it; --help-all prints all of it\n(%s).\n", size)
	case n == 1:
		fmt.Fprintf(own, "\nThe command below is not listed: its help, and even its name, is more than an\nAI agent's --help prints at once. --help on it prints its help; --help-all\nprints all of it (%s).\n", size)
	default:
		fmt.Fprintf(own, "\nThe %d commands below are not listed: their help, and even the list of them,\nis more than an AI agent's --help prints at once. --help on a command above\nprints its help with the commands below it; --help-all prints all of it\n(%s).\n", n, size)
	}
	_, err := own.WriteTo(w)
	return err
}

// commandMap is every visible command below c, a line for each shape of them,
// in the order c's help lists them. The commands below one with none below
// them share a line, and so do commands whose commands below have the same
// names: a choice of names is in braces, so that "app {user,team} {get,ls}"
// is app user get, app user ls, app team get and app team ls.
func commandMap(c *Command) []string {
	lines := []string{}
	var walk func(c *Command, path string)
	walk = func(c *Command, path string) {
		leaves := []string{}
		shapes := []string{}
		alike := map[string][]*Command{}
		for _, group := range c.Commands.Visible().ByCategory() {
			for _, sub := range group {
				if len(sub.Commands.Visible()) == 0 {
					leaves = append(leaves, sub.Name)
					continue
				}
				s := shapeOf(sub)
				if _, ok := alike[s]; !ok {
					shapes = append(shapes, s)
				}
				alike[s] = append(alike[s], sub)
			}
		}
		if len(leaves) > 0 {
			lines = append(lines, path+" "+braces(leaves))
		}
		for _, s := range shapes {
			subs := alike[s]
			names := make([]string, len(subs))
			for i, sub := range subs {
				names[i] = sub.Name
			}
			walk(subs[0], path+" "+braces(names))
		}
	}
	walk(c, commandPath(c))
	return lines
}

// shapeOf is the names of the visible commands below c and of theirs, in an
// order of its own: commands with the same commands below them have the same
// shape, whatever they are called themselves.
func shapeOf(c *Command) string {
	vs := []string{}
	for _, sub := range c.Commands.Visible() {
		vs = append(vs, sub.Name+shapeOf(sub))
	}
	slices.Sort(vs)
	return "{" + strings.Join(vs, ",") + "}"
}

// braces is one name, or a choice of several: "{a,b}".
func braces(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return "{" + strings.Join(names, ",") + "}"
}

// countBelow is the number of visible commands below c.
func countBelow(c *Command) int {
	n := 0
	for _, sub := range c.Commands.Visible() {
		n += 1 + countBelow(sub)
	}
	return n
}

// kb is n bytes as the help says it.
func kb(n int) string {
	return fmt.Sprintf("%d KB", (n+500)/1000)
}

// countWriter counts what is written to it.
type countWriter struct {
	n int
}

func (w *countWriter) Write(p []byte) (int, error) {
	w.n += len(p)
	return len(p), nil
}

// errCapped is what a capWriter answers once it would hold more than its max.
var errCapped = errors.New("xli: help over its cap")

// capWriter keeps what is written to it, up to max bytes.
type capWriter struct {
	buf bytes.Buffer
	max int
}

func (w *capWriter) Write(p []byte) (int, error) {
	if w.buf.Len()+len(p) > w.max {
		return 0, errCapped
	}
	return w.buf.Write(p)
}
