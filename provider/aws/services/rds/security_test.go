package rds

import "testing"

func TestDBSecurityFindings_FlagsPubliclyAccessible(t *testing.T) {
	id := "orders-db"
	public := true
	def := &dbDefinition{identifier: &id, publiclyAccessible: &public}

	findings := dbSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleDBPubliclyAccessible {
		t.Fatalf("expected 1 %s finding, got %+v", ruleDBPubliclyAccessible, findings)
	}
	if findings[0].ResourceID != "orders-db" {
		t.Errorf("expected ResourceID to be the identifier, got %q", findings[0].ResourceID)
	}
}

func TestDBSecurityFindings_NoFindingWhenPrivate(t *testing.T) {
	id := "orders-db"
	private := false
	def := &dbDefinition{identifier: &id, publiclyAccessible: &private}

	if findings := dbSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a private instance, got %+v", findings)
	}
}

func TestDBSecurityFindings_NoFindingWhenUnknown(t *testing.T) {
	if findings := dbSecurityFindings(&dbDefinition{}); len(findings) != 0 {
		t.Fatalf("expected no findings when publiclyAccessible is nil, got %+v", findings)
	}
}
