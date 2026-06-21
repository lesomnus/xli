package arg

import (
	"time"
)

type Duration = Base[time.Duration, DurationParser]

type DurationParser struct{}

func (DurationParser) Parse(rest []string) (time.Duration, int, error) {
	v, err := time.ParseDuration(rest[0])
	return v, 1, err
}

func (DurationParser) String() string {
	return "duration"
}
