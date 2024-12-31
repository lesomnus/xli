package tab

import (
	"fmt"
	"io"
)

type ZshTab struct {
	io.Writer
}

func NewZshTab(w io.Writer) *ZshTab {
	return &ZshTab{Writer: w}
}

func (t *ZshTab) Value(v string) {
	fmt.Fprintf(t, "%s\n", v)
}

func (t *ZshTab) ValueD(v string, desc string) {
	fmt.Fprintf(t, "%s:%s\n", v, desc)
}
