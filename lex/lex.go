package lex

import (
	"errors"
	"strings"
)

func Lex(v string) Token {
	if !strings.HasPrefix(v, "-") {
		return Arg(v)
	}

	i := strings.IndexFunc(v, func(r rune) bool {
		return r != '-'
	})
	if i > 2 {
		return &Err{
			raw: v,
			err: errors.New("three dashes"),
		}
	}
	if v == "--" {
		return EndOfCommand("")
	}

	j := strings.IndexFunc(v, func(r rune) bool {
		return r == '='
	})
	if j > 0 {
		arg := Arg(v[j+1:])
		return &Flag{
			raw:  v,      // --flag=arg
			name: v[i:j], // flag
			arg:  &arg,   // arg
		}
	} else {
		return &Flag{
			raw:  v,     // --flag
			name: v[i:], // flag
		}
	}
}
