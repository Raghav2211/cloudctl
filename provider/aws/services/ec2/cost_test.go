package ec2

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"errors"
	"testing"
)

func TestInstanceIdleFinding_FlagsLowCPU(t *testing.T) {
	id := "i-abc123"
	avg := 1.2
	s := instanceStatisticsOutput{instanceId: &id, Average: &avg, CPUStatus: CPU_LOW}

	f := instanceIdleFinding(s)
	if f == nil || f.Rule != ruleEC2IdleLowCPU || f.ResourceID != "i-abc123" {
		t.Fatalf("expected a %s finding, got %+v", ruleEC2IdleLowCPU, f)
	}
}

func TestInstanceIdleFinding_NoFindingForHighCPU(t *testing.T) {
	id := "i-abc123"
	avg := 80.0
	s := instanceStatisticsOutput{instanceId: &id, Average: &avg, CPUStatus: CPU_HIGH}

	if f := instanceIdleFinding(s); f != nil {
		t.Fatalf("expected no finding for high CPU, got %+v", f)
	}
}

func TestInstanceIdleFinding_NoFindingOnAPIError(t *testing.T) {
	id := "i-abc123"
	s := instanceStatisticsOutput{instanceId: &id, apiError: ctlaws.NewErrorInfo(errors.New("boom"), viewer.ERROR, nil)}

	if f := instanceIdleFinding(s); f != nil {
		t.Fatalf("expected no finding when the CloudWatch fetch failed, got %+v", f)
	}
}
