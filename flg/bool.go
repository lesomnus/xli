package flg

import (
	"fmt"
)

type Switch = Flag[bool, SwitchParser]

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
