package lambda

import "testing"

func TestFunctionHealth(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  string
	}{
		{"active", "Active", "healthy"},
		{"pending", "Pending", "degraded"},
		{"failed", "Failed", "unhealthy"},
		{"inactive", "Inactive", "degraded"},
		{"unrecognized", "SomethingNew", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			def := &functionDefinition{state: c.state}
			got := functionHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("state %q: expected %q, got %q (reasons: %v)", c.state, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestFunctionHealth_EmptyStateIsUnknown(t *testing.T) {
	got := functionHealth(&functionDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for empty state, got %q", got.Status)
	}
}
