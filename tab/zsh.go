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

func (t *ZshTab) Value(v string, desc string) {
	fmt.Fprintf(t, "%s:%s\n", v, desc)
}
