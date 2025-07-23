package cmd_test

import (
	"io"
	"testing"

	"github.com/lesomnus/xli/internal/examples/completion/cmd"
)

func BenchmarkExampleCompletion(b *testing.B) {
	c := cmd.NewExampleCompletionCmd()
	c.Writer = io.Discard
	for b.Loop() {
		c.Run(b.Context(), []string{"echo", "42"})
	}
}
