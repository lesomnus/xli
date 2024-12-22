package flag

import (
	"fmt"
	"strconv"
)

type Int = Flag[int, IntParser]
type Int32 = Flag[int32, Int32Parser]
type Int64 = Flag[int64, Int64Parser]

type Uint = Flag[uint, UintParser]
type Uint32 = Flag[uint32, Uint32Parser]
type Uint64 = Flag[uint64, Uint64Parser]

type IntParser struct{}

func (IntParser) Parse(s string) (int, error) {
	v, err := strconv.ParseInt(s, 10, 64)
	return int(v), err
}

func (IntParser) ToString(v int) string {
	return fmt.Sprintf("%d", v)
}

type Int32Parser struct{}

func (Int32Parser) Parse(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

func (Int32Parser) ToString(v int32) string {
	return fmt.Sprintf("%d", v)
}

type Int64Parser struct{}

func (Int64Parser) Parse(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func (Int64Parser) ToString(v int64) string {
	return fmt.Sprintf("%d", v)
}

type UintParser struct{}

func (UintParser) Parse(s string) (uint, error) {
	v, err := strconv.ParseUint(s, 10, 64)
	return uint(v), err
}

func (UintParser) ToString(v uint) string {
	return fmt.Sprintf("%d", v)
}

type Uint32Parser struct{}

func (Uint32Parser) Parse(s string) (uint32, error) {
	v, err := strconv.ParseUint(s, 10, 32)
	return uint32(v), err
}

func (Uint32Parser) ToString(v uint32) string {
	return fmt.Sprintf("%d", v)
}

type Uint64Parser struct{}

func (Uint64Parser) Parse(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func (Uint64Parser) ToString(v uint64) string {
	return fmt.Sprintf("%d", v)
}
