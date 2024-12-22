package flag

import (
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"
)

type Int = Flag[int, IntParser]
type Int32 = Flag[int32, Int32Parser]
type Int64 = Flag[int64, Int64Parser]

type Uint = Flag[uint, UintParser]
type Uint32 = Flag[uint32, Uint32Parser]
type Uint64 = Flag[uint64, Uint64Parser]

type intParserBase[T constraints.Integer] struct{}

func (intParserBase[T]) ToString(v T) string {
	return fmt.Sprintf("%d", v)
}

func (intParserBase[T]) String() string {
	return fmt.Sprintf("%T", *new(T))
}

type IntParser struct{ intParserBase[int] }

func (IntParser) Parse(s string) (int, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	return int(v), err
}

type Int32Parser struct{ intParserBase[int32] }

func (Int32Parser) Parse(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

type Int64Parser struct{ intParserBase[int64] }

func (Int64Parser) Parse(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

type UintParser struct{ intParserBase[uint] }

func (UintParser) Parse(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

type Uint32Parser struct{ intParserBase[uint32] }

func (Uint32Parser) Parse(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err
}

type Uint64Parser struct{ intParserBase[uint64] }

func (Uint64Parser) Parse(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}
