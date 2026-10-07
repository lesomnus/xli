package comp

import (
	"fmt"
	"io"
	"strings"

	"github.com/lesomnus/xli/tab"
)

// Writer is the tab.Tab that writes candidates in the line format every
// generated shell script decodes, the same for each shell. Each emitted line
// is one of:
//
//	v<sep><group><sep><entry>   a candidate ("value" or "value:desc"; a ":" in
//	                            the value is escaped as "\:", as zsh expects)
//	f<sep><pattern>             request file completion (pattern may be empty)
//	d<sep>                      request directory completion
type Writer struct {
	io.Writer
	group string
}

// NewWriter is a Writer that writes to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{Writer: w}
}

func (t *Writer) Value(v string) {
	t.candidate(escapeValue(v))
}

func (t *Writer) ValueD(v string, desc string) {
	t.candidate(fmt.Sprintf("%s:%s", escapeValue(v), desc))
}

// escapeValue escapes ":" so it is not taken as the value/description
// separator.
func escapeValue(v string) string {
	return strings.ReplaceAll(v, ":", `\:`)
}

func (t *Writer) Group(name string) tab.Tab {
	return &Writer{Writer: t.Writer, group: name}
}

func (t *Writer) Files(pattern string) {
	fmt.Fprintf(t, "f%s%s\n", Sep, pattern)
}

func (t *Writer) Dirs() {
	fmt.Fprintf(t, "d%s\n", Sep)
}

func (t *Writer) candidate(entry string) {
	fmt.Fprintf(t, "v%s%s%s%s\n", Sep, t.group, Sep, entry)
}
