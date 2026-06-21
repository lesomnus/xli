package flg_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/lesomnus/xli/flg"
	"github.com/lesomnus/xli/internal/x"
)

// yesParser is a value-less parser that is not flg.SwitchParser; it exercises
// the interface-based no-value detection (a custom switch type).
type yesParser struct{}

func (yesParser) Parse(s string) (bool, error) {
	return s != "false", nil
}
func (yesParser) ToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
func (yesParser) String() string {
	return ""
}
func (yesParser) NoValue() bool {
	return true
}

func TestNoValue(t *testing.T) {
	t.Run("switch reports NoValue", x.F(func(x x.X) {
		var f flg.Flag = &flg.Switch{Name: "v"}
		x.True(f.NoValue())
	}))
	t.Run("value flag does not report NoValue", x.F(func(x x.X) {
		var f flg.Flag = &flg.String{Name: "name"}
		x.False(f.NoValue())
	}))
	t.Run("switch without explicit value parses as true", x.F(func(x x.X) {
		v := false
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Switch{Name: "on", Value: &v},
			},
		}

		err := c.Run(t.Context(), []string{"--on"})
		x.NoError(err)
		x.True(v)
	}))
	t.Run("custom no-value flag works as a switch", x.F(func(x x.X) {
		v := false
		c := &xli.Command{
			Flags: flg.Flags{
				&flg.Base[bool, yesParser]{Name: "y", Value: &v},
			},
		}

		err := c.Run(t.Context(), []string{"--y"})
		x.NoError(err)
		x.True(v)
	}))
}
