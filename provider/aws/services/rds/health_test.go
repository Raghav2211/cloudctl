package rds

import "testing"

func TestDBHealth(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"available", "available", "healthy"},
		{"backing-up", "backing-up", "degraded"},
		{"modifying", "modifying", "degraded"},
		{"failed", "failed", "unhealthy"},
		{"storage-full", "storage-full", "unhealthy"},
		{"stopped", "stopped", "unknown"},
		{"unrecognized", "something-new", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := c.status
			def := &dbDefinition{status: &status}
			got := dbHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("status %q: expected %q, got %q (reasons: %v)", c.status, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestDBHealth_NilStatusIsUnknown(t *testing.T) {
	got := dbHealth(&dbDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for nil status, got %q", got.Status)
	}
}
