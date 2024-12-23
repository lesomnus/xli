package flg

import (
	"fmt"
)

type String = Base[string, StringParser]

type StringParser struct{}

func (StringParser) Parse(s string) (string, error) {
	return s, nil
}

func (StringParser) ToString(v string) string {
	return fmt.Sprintf("%q", v)
}

func (StringParser) String() string {
	return "string"
}
