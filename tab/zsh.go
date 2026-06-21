package tab

import (
	"fmt"
	"io"
)

// zshSep separates fields on each emitted line. It is a non-printing byte that
// survives shell command substitution (unlike NUL).
const zshSep = "\x1f"

// Each emitted line is one of:
//
//	v<sep><group><sep><entry>   a candidate ("value" or "value:desc")
//	f<sep><pattern>             request file completion (pattern may be empty)
//	d<sep>                      request directory completion
type ZshTab struct {
	io.Writer
	group string
}

func NewZshTab(w io.Writer) *ZshTab {
	return &ZshTab{Writer: w}
}

func (t *ZshTab) Value(v string) {
	t.candidate(v)
}

func (t *ZshTab) ValueD(v string, desc string) {
	t.candidate(fmt.Sprintf("%s:%s", v, desc))
}

func (t *ZshTab) Group(name string) Tab {
	return &ZshTab{Writer: t.Writer, group: name}
}

func (t *ZshTab) Files(pattern string) {
	fmt.Fprintf(t, "f%s%s\n", zshSep, pattern)
}

func (t *ZshTab) Dirs() {
	fmt.Fprintf(t, "d%s\n", zshSep)
}

func (t *ZshTab) candidate(entry string) {
	fmt.Fprintf(t, "v%s%s%s%s\n", zshSep, t.group, zshSep, entry)
}
