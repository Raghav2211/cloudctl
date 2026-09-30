package vpc

import "testing"

func TestVPCHealth(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  string
	}{
		{"available", "available", "healthy"},
		{"pending", "pending", "degraded"},
		{"unrecognized", "something-new", "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state := c.state
			def := &vpcDefinition{state: &state}
			got := vpcHealth(def)
			if string(got.Status) != c.want {
				t.Errorf("state %q: expected %q, got %q (reasons: %v)", c.state, c.want, got.Status, got.Reasons)
			}
		})
	}
}

func TestVPCHealth_NilStateIsUnknown(t *testing.T) {
	got := vpcHealth(&vpcDefinition{})
	if got.Status != "unknown" {
		t.Errorf("expected unknown for nil state, got %q", got.Status)
	}
}
