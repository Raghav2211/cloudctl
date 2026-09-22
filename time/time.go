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

// GetTZ never returns nil: an unrecognized or empty identifier falls back
// to UTC rather than silently returning a nil *Timezone, which would panic
// the next time AdaptTimezone is called on it (confirmed live: EC2's
// executors hardcoded GetTZ(""), which crashed on the first real instance
// with a LaunchTime — see ADR-012).
func GetTZ(tz_short_identifier string) *Timezone {
	if tz, ok := supportedTimezones[tz_short_identifier]; ok {
		return tz
	}
	return supportedTimezones["utc"]
}

func (tz *Timezone) AdaptTimezone(t *time.Time) *time.Time {
	loc, _ := time.LoadLocation(tz.identifier)
	adaptTzTime := t.In(loc)
	return &adaptTzTime
}
