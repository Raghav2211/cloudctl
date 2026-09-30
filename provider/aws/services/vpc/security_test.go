package vpc

import "testing"

func TestVPCSecurityFindings_FlagsDefaultVPC(t *testing.T) {
	id := "vpc-abc123"
	def := &vpcDefinition{id: &id, isDefault: true}

	findings := vpcSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleVPCDefaultInUse {
		t.Fatalf("expected 1 %s finding, got %+v", ruleVPCDefaultInUse, findings)
	}
}

func TestVPCSecurityFindings_NoFindingForCustomVPC(t *testing.T) {
	id := "vpc-abc123"
	def := &vpcDefinition{id: &id, isDefault: false}

	if findings := vpcSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a non-default VPC, got %+v", findings)
	}
}
