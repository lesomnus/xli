package flg

import (
	"time"
)

type Duration = Base[time.Duration, DurationParser]

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
