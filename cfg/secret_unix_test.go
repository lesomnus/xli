//go:build unix

package cfg_test

import (
	"path/filepath"
	"syscall"
	"testing"

	"github.com/lesomnus/xli/cfg"
	"github.com/lesomnus/xli/internal/x"
)

func TestSecretFileNotRegular(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skip("no FIFO here:", err)
	}
	c := &Secrets{}
	s, err := cfg.New("app", c).Load(write(t, "password: ${file:"+fifo+"}\n"), nil)
	x := x.New(t)
	x.NoError(err, "a FIFO would block the read")
	x.ErrorContains(s.Warnings[0], "not a regular file")
}
