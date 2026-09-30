package ec2

import "testing"

func TestInstanceHealth(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  string
	}{
		{"running", "running", "healthy"},
		{"pending", "pending", "degraded"},
		{"stopping", "stopping", "degraded"},
		{"shutting-down", "shutting-down", "degraded"},
		{"stopped", "stopped", "unknown"},
		{"terminated", "terminated", "unknown"},
		{"unrecognized", "something-new", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := c.state
			def := &instanceDefinition{summary: &instanceSummary{state: &state}}
			got := instanceHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("state %q: expected %q, got %q (reasons: %v)", c.state, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestInstanceHealth_NilSummaryIsUnknown(t *testing.T) {
	got := instanceHealth(&instanceDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for nil summary, got %q", got.Status)
	}
	if len(got.Reasons) == 0 {
		t.Error("expected a reason explaining the unknown status")
	}
}
