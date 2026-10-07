package flg

import (
	"time"
)

// Duration is a flag that takes a time.Duration, written as
// time.ParseDuration reads one: "1h30m", "250ms".
type Duration = Base[time.Duration, DurationParser]

// DurationParser reads a duration as time.ParseDuration does.
type DurationParser struct{}

func (DurationParser) Parse(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func (DurationParser) ToString(v time.Duration) string {
	return v.String()
}

func (DurationParser) String() string {
	return "duration"
}
