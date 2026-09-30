package dynamodb

import "testing"

func TestTableHealth(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"active", "ACTIVE", "healthy"},
		{"creating", "CREATING", "degraded"},
		{"updating", "UPDATING", "degraded"},
		{"deleting", "DELETING", "degraded"},
		{"archived", "ARCHIVED", "unhealthy"},
		{"unrecognized", "SOMETHING_NEW", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := c.status
			def := &tableDefinition{status: &status}
			got := tableHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("status %q: expected %q, got %q (reasons: %v)", c.status, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestTableHealth_NilStatusIsUnknown(t *testing.T) {
	got := tableHealth(&tableDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for nil status, got %q", got.Status)
	}
	if len(got.Reasons) == 0 {
		t.Error("expected a reason explaining the unknown status")
	}
}
