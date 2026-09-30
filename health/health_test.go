package health

import "testing"

func TestOf_HealthyNeedsNoReasons(t *testing.T) {
	a := Of(Healthy)
	if a.Status != Healthy {
		t.Errorf("expected Healthy, got %q", a.Status)
	}
	if len(a.Reasons) != 0 {
		t.Errorf("expected no reasons, got %v", a.Reasons)
	}
}

func TestOf_CarriesReasons(t *testing.T) {
	a := Of(Degraded, "still starting")
	if a.Status != Degraded {
		t.Errorf("expected Degraded, got %q", a.Status)
	}
	if len(a.Reasons) != 1 || a.Reasons[0] != "still starting" {
		t.Errorf("expected reasons [still starting], got %v", a.Reasons)
	}
}
