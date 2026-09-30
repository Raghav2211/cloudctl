package lambda

import "testing"

func TestFunctionIdleFindings_FlagsNoInvocations(t *testing.T) {
	invocations := 0.0
	stats := &functionStatistics{functionName: "orders-handler", invocations: &invocations}

	findings := functionIdleFindings(stats)
	if len(findings) != 1 || findings[0].Rule != ruleFunctionIdleNoInvocations {
		t.Fatalf("expected 1 %s finding, got %+v", ruleFunctionIdleNoInvocations, findings)
	}
}

func TestFunctionIdleFindings_NoFindingForActiveFunction(t *testing.T) {
	invocations := 500.0
	stats := &functionStatistics{functionName: "orders-handler", invocations: &invocations}

	if findings := functionIdleFindings(stats); len(findings) != 0 {
		t.Fatalf("expected no findings for an active function, got %+v", findings)
	}
}

func TestFunctionIdleFindings_NoFindingWhenDataMissing(t *testing.T) {
	if findings := functionIdleFindings(&functionStatistics{functionName: "orders-handler"}); len(findings) != 0 {
		t.Fatalf("expected no findings when metrics are unavailable, got %+v", findings)
	}
}
