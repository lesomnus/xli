package arg

import (
	"context"
	"fmt"
	"strconv"

	"golang.org/x/exp/constraints"
)

type Int = Base[int, IntParser]
type Int32 = Base[int32, Int32Parser]
type Int64 = Base[int64, Int64Parser]

type Uint = Base[uint, UintParser]
type Uint32 = Base[uint32, Uint32Parser]
type Uint64 = Base[uint64, Uint64Parser]

type intParserBase[T constraints.Integer] struct{}

func (intParserBase[T]) String() string {
	return fmt.Sprintf("%T", *new(T))
}

type IntParser struct{ intParserBase[int] }

func (IntParser) Parse(ctx context.Context, rest []string) (int, int, error) {
	v, err := strconv.ParseInt(rest[0], 10, 64)
	return int(v), 1, err
}

type Int32Parser struct{ intParserBase[int32] }

func (Int32Parser) Parse(ctx context.Context, rest []string) (int32, int, error) {
	v, err := strconv.ParseInt(rest[0], 10, 32)
	return int32(v), 1, err
}

type Int64Parser struct{ intParserBase[int64] }

func (Int64Parser) Parse(ctx context.Context, rest []string) (int64, int, error) {
	v, err := strconv.ParseInt(rest[0], 10, 64)
	return int64(v), 1, err
}

type UintParser struct{ intParserBase[uint] }

func (UintParser) Parse(ctx context.Context, rest []string) (uint, int, error) {
	v, err := strconv.ParseUint(rest[0], 10, 64)
	return uint(v), 1, err
}

type Uint32Parser struct{ intParserBase[uint32] }

func (Uint32Parser) Parse(ctx context.Context, rest []string) (uint32, int, error) {
	v, err := strconv.ParseUint(rest[0], 10, 32)
	return uint32(v), 1, err
}

type Uint64Parser struct{ intParserBase[uint64] }

func (Uint64Parser) Parse(ctx context.Context, rest []string) (uint64, int, error) {
	v, err := strconv.ParseUint(rest[0], 10, 64)
	return uint64(v), 1, err
}
