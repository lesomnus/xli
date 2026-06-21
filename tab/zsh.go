package tab

import (
	"fmt"
	"io"
)

// zshSep separates the group from the candidate on each emitted line. It is a
// non-printing byte that survives shell command substitution (unlike NUL).
const zshSep = "\x1f"

type ZshTab struct {
	io.Writer
	group string
}

func NewZshTab(w io.Writer) *ZshTab {
	return &ZshTab{Writer: w}
}

func (t *ZshTab) Value(v string) {
	t.emit(v)
}

func (t *ZshTab) ValueD(v string, desc string) {
	t.emit(fmt.Sprintf("%s:%s", v, desc))
}

func (t *ZshTab) Group(name string) Tab {
	return &ZshTab{Writer: t.Writer, group: name}
}

// emit writes one "<group><sep><entry>" line, where entry is "value" or
// "value:desc". The group may be empty.
func (t *ZshTab) emit(entry string) {
	fmt.Fprintf(t, "%s%s%s\n", t.group, zshSep, entry)
}
