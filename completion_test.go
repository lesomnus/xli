package xli_test

import (
	"testing"

	"github.com/lesomnus/xli"
	"github.com/stretchr/testify/require"
)

func TestNormalizeCompletionArgs(t *testing.T) {
	tcs := []struct {
		desc     string
		args     []string
		curr     string
		buff     string
		expected []string
	}{
		{
			"cursor at the middle of the arg",
			// $ foo bar
			//        ^
			[]string{"foo", "bar"}, "bar", "o b",
			[]string{"foo"},
		},
		{
			"cursor at the end of the arg",
			// $ foo bar
			//          ^
			[]string{"foo", "bar"}, "bar", "bar",
			[]string{"foo"},
		},
		{
			"cursor at the start of the new arg",
			// $ foo bar
			//           ^
			[]string{"foo", "bar"}, "", "",
			[]string{"foo", "bar"},
		},
		{
			"cursor at the middle of the flag",
			// $ foo --bar
			//          ^
			[]string{"foo", "--bar"}, "--bar", "o --b",
			[]string{"foo", "--"},
		},
		{
			"cursor at the end of the flag",
			// $ foo --bar
			//            ^
			[]string{"foo", "--bar"}, "--bar", "--bar",
			[]string{"foo", "--"},
		},
		{
			"cursor at the start of new arg next to the flag",
			// $ foo --bar
			//             ^
			[]string{"foo", "--bar"}, "", "",
			[]string{"foo", "--bar"},
		},
		{
			"cursor at the middle of the flag with value",
			// $ foo --bar=baz
			//          ^
			[]string{"foo", "--bar=baz"}, "--bar=baz", "o --b",
			[]string{"foo", "--"},
		},
		{
			"cursor at the middle of the flag value",
			// $ foo --bar=baz
			//              ^
			[]string{"foo", "--bar=baz"}, "--bar=baz", "o --bar=b",
			[]string{"foo", "--bar="},
		},
		{
			"cursor at the end of the flag value",
			// $ foo --bar=baz
			//                ^
			[]string{"foo", "--bar=baz"}, "--bar=baz", "--bar=baz",
			[]string{"foo", "--bar="},
		},
		{
			"cursor at the middle of the flag value with equal sign",
			// $ foo --bar=baz=qux
			//                  ^
			[]string{"foo", "--bar=baz=qux"}, "--bar=baz=qux", "o --bar=baz=qux",
			[]string{"foo", "--bar="},
		},
		{
			"cursor at the end of the flag value with equal sign",
			// $ foo --bar=baz=qux
			//                    ^
			[]string{"foo", "--bar=baz=qux"}, "--bar=baz=qux", "--bar=baz=qux",
			[]string{"foo", "--bar="},
		},
		{
			"cursor at the start of new arg next to the flag value with equal sign",
			// $ foo --bar=baz=qux
			//                     ^
			[]string{"foo", "--bar=baz=qux"}, "", "",
			[]string{"foo", "--bar=baz=qux"},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			args := xli.NormalizeCompletionArgs(tc.args, tc.curr, tc.buff)
			require.Equal(t, tc.expected, args)
		})
	}
}
