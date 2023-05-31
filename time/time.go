package time

import (
	"time"
)

type Timezone struct {
	identifier string
}

var (
	supportedTimezones = map[string]*Timezone{
		"utc": {
			identifier: "UTC",
		},
		"los_angeles": {
			identifier: "America/Los_Angeles",
		},
		"tokyo": {
			identifier: "Asia/Tokyo",
		},
	}
)

func GetTZ(tz_short_identifier string) *Timezone {
	return supportedTimezones[tz_short_identifier]
}

func (tz *Timezone) AdaptTimezone(t *time.Time) *time.Time {
	loc, _ := time.LoadLocation(tz.identifier)
	adaptTzTime := t.In(loc)
	return &adaptTzTime
}
