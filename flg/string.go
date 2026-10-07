package flg

import (
	"fmt"
)

// String is a flag that takes a string.
type String = Base[string, StringParser]

// StringParser takes the text as it is.
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
