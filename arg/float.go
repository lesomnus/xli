package arg

import (
	"strconv"
)

// Float32 is a argument that takes a float32, and Float64 a float64.
type Float32 = Base[float32, Float32Parser]
type Float64 = Base[float64, Float64Parser]

// Float32Parser reads a float32 as strconv.ParseFloat does.
type Float32Parser struct{}

func (Float32Parser) Parse(rest []string) (float32, int, error) {
	v, err := strconv.ParseFloat(rest[0], 32)
	return float32(v), 1, err
}

func (Float32Parser) String() string {
	return "float32"
}

// Float64Parser reads a float64 as strconv.ParseFloat does.
type Float64Parser struct{}

func (Float64Parser) Parse(rest []string) (float64, int, error) {
	v, err := strconv.ParseFloat(rest[0], 64)
	return v, 1, err
}

func (Float64Parser) String() string {
	return "float64"
}
