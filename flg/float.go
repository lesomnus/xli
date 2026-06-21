package flg

import (
	"strconv"
)

type Float32 = Base[float32, Float32Parser]
type Float64 = Base[float64, Float64Parser]

type Float32Parser struct{}

func (Float32Parser) Parse(s string) (float32, error) {
	v, err := strconv.ParseFloat(s, 32)
	return float32(v), err
}

func (Float32Parser) ToString(v float32) string {
	return strconv.FormatFloat(float64(v), 'g', -1, 32)
}

func (Float32Parser) String() string {
	return "float32"
}

type Float64Parser struct{}

func (Float64Parser) Parse(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

func (Float64Parser) ToString(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func (Float64Parser) String() string {
	return "float64"
}
