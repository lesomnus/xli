package arg

import (
	"time"
)

// Duration is a argument that takes a time.Duration, written as
// time.ParseDuration reads one: "1h30m", "250ms".
type Duration = Base[time.Duration, DurationParser]

// DurationParser reads a duration as time.ParseDuration does.
type DurationParser struct{}

func (DurationParser) Parse(rest []string) (time.Duration, int, error) {
	v, err := time.ParseDuration(rest[0])
	return v, 1, err
}

func (DurationParser) String() string {
	return "duration"
}
