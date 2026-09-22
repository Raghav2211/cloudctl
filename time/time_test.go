package time

import "testing"

// Regression test: GetTZ("") used to return a nil *Timezone (a map lookup
// miss), which crashed the next time AdaptTimezone was called on it. This
// was hit live: every EC2 executor constructor hardcoded GetTZ(""), and it
// never surfaced until a real account with a real running instance (which
// has a non-nil LaunchTime) was queried.
func TestGetTZ_NeverReturnsNil(t *testing.T) {
	for _, id := range []string{"", "utc", "los_angeles", "tokyo", "not-a-real-timezone"} {
		if tz := GetTZ(id); tz == nil {
			t.Errorf("GetTZ(%q) returned nil", id)
		}
	}
}

func TestGetTZ_KnownIdentifiersResolveCorrectly(t *testing.T) {
	cases := map[string]string{
		"utc":         "UTC",
		"los_angeles": "America/Los_Angeles",
		"tokyo":       "Asia/Tokyo",
	}
	for id, wantIdentifier := range cases {
		got := GetTZ(id)
		if got.identifier != wantIdentifier {
			t.Errorf("GetTZ(%q).identifier = %q, want %q", id, got.identifier, wantIdentifier)
		}
	}
}

func TestGetTZ_UnknownIdentifierFallsBackToUTC(t *testing.T) {
	if got := GetTZ("not-a-real-timezone"); got.identifier != "UTC" {
		t.Errorf("expected an unrecognized identifier to fall back to UTC, got %q", got.identifier)
	}
}
