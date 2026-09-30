package eks

import "testing"

func TestClusterHealth(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"active", "ACTIVE", "healthy"},
		{"creating", "CREATING", "degraded"},
		{"updating", "UPDATING", "degraded"},
		{"deleting", "DELETING", "unhealthy"},
		{"failed", "FAILED", "unhealthy"},
		{"unrecognized", "SOMETHING_NEW", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status := c.status
			def := &clusterDefinition{status: &status}
			got := clusterHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("status %q: expected %q, got %q (reasons: %v)", c.status, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestClusterHealth_DegradesForUnhealthyNodeGroup(t *testing.T) {
	active := "ACTIVE"
	degrading := "DEGRADED"
	name := "workers"
	def := &clusterDefinition{
		status:     &active,
		nodeGroups: []*nodeGroupSummary{{name: &name, status: &degrading}},
	}

	got := clusterHealth(def)
	if got.Status != "degraded" {
		t.Errorf("expected a degraded node group to degrade overall cluster health, got %q", got.Status)
	}
	if len(got.Reasons) != 1 {
		t.Errorf("expected 1 reason citing the node group, got %v", got.Reasons)
	}
}

func TestClusterHealth_NilStatusIsUnknown(t *testing.T) {
	got := clusterHealth(&clusterDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for nil status, got %q", got.Status)
	}
}
