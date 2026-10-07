package flg

import (
	"fmt"
)

// Switch is a flag that takes no value: given, it is true. It may be given
// as --name=false to set it false, as against a Default of true.
type Switch = Base[bool, SwitchParser]

// SwitchParser reads "true", "false", or nothing, which is true; it takes no
// value of its own, so a switch is given as --name or --name=false.
type SwitchParser struct{}

func (SwitchParser) Parse(s string) (bool, error) {
	switch s {
	case "false":
		return false, nil
	case "", "true":
		return true, nil
	default:
		return false, fmt.Errorf(`invalid value: expected "true" or "false" but %s`, s)
	}
}

func (SwitchParser) ToString(v bool) string {
	if v {
		return "true"
	} else {
		return "false"
	}
}

func (SwitchParser) String() string {
	return ""
}

func (SwitchParser) NoValue() bool {
	return true
}
