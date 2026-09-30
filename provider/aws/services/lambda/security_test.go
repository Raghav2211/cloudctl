package lambda

import "testing"

func TestFunctionSecurityFindings_FlagsDeprecatedRuntime(t *testing.T) {
	name := "orders-handler"
	def := &functionDefinition{name: &name, runtime: "python3.7"}

	findings := functionSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleLambdaDeprecatedRuntime {
		t.Fatalf("expected 1 %s finding, got %+v", ruleLambdaDeprecatedRuntime, findings)
	}
}

func TestFunctionSecurityFindings_NoFindingForCurrentRuntime(t *testing.T) {
	name := "orders-handler"
	def := &functionDefinition{name: &name, runtime: "python3.12"}

	if findings := functionSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a current runtime, got %+v", findings)
	}
}
