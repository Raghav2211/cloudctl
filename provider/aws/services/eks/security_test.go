package eks

import "testing"

func TestClusterSecurityFindings_FlagsPublicEndpoint(t *testing.T) {
	name := "prod-cluster"
	def := &clusterDefinition{name: &name, endpointPublicAccess: true, publicAccessCidrs: []string{"0.0.0.0/0"}}

	findings := clusterSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleEKSPublicEndpoint {
		t.Fatalf("expected 1 %s finding, got %+v", ruleEKSPublicEndpoint, findings)
	}
}

func TestClusterSecurityFindings_NoFindingWhenPrivateOnly(t *testing.T) {
	name := "prod-cluster"
	def := &clusterDefinition{name: &name, endpointPublicAccess: false, endpointPrivateAccess: true}

	if findings := clusterSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a private-only endpoint, got %+v", findings)
	}
}
