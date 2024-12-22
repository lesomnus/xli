package flag

import (
	"fmt"
)

type String = Flag[string, StringParser]

type StringParser struct{}

func (StringParser) Parse(v string) (string, error) {
	return v, nil
}

func (StringParser) ToString(v string) string {
	return fmt.Sprintf("%q", v)
}
