package flag

import (
	"fmt"
)

type Switch = Flag[bool, SwitchParser]

type SwitchParser struct{}

func (SwitchParser) Parse(v string) (bool, error) {
	switch v {
	case "false":
		return false, nil
	case "", "true":
		return true, nil
	default:
		return false, fmt.Errorf(`invalid value: expected "true" or "false" but %s`, v)
	}
}

func (SwitchParser) ToString(v bool) string {
	if v {
		return "true"
	} else {
		return "false"
	}
}
