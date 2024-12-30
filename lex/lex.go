package lex

import (
	"errors"
	"strings"
)

// Lex classifies the input string `v` and returns a `Token“ based on its format.
// It distinguishes between arguments, flags, and end-of-command markers.
//
// If the input string does not start with a dash, it is treated as an argument.
// If the input string is a single dash ("-") or double dash ("--"), it is treated
// as an argument or end-of-command marker respectively.
//
// For strings starting with one or two dashes followed by other characters, it
// checks for the presence of an equal sign to differentiate between flags with
// and without arguments. If an equal sign is found, the part after the equal sign
// is treated as the flag's argument.
//
// If the input string contains more than two leading dashes, an error token is returned.
func Lex(v string) Token {
	if !strings.HasPrefix(v, "-") {
		return Arg(v)
	}
	switch v {
	case "-":
		return Arg("-")
	case "--":
		return EndOfCommand("")
	}

	i := strings.IndexFunc(v, func(r rune) bool {
		return r != '-'
	})
	if i == -1 || i > 2 {
		return &Err{
			raw: v,
			err: errors.New("too many dashes"),
		}
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
