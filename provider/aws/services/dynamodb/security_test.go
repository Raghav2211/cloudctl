package dynamodb

import "testing"

func TestTableSecurityFindings_FlagsDefaultEncryptionKey(t *testing.T) {
	name := "orders"
	enc := defaultOwnedKeyEncryptionType
	def := &tableDefinition{name: &name, encryptionType: &enc}

	findings := tableSecurityFindings(def)
	if len(findings) != 1 || findings[0].Rule != ruleDynamoDBDefaultEncryptionKey {
		t.Fatalf("expected 1 %s finding, got %+v", ruleDynamoDBDefaultEncryptionKey, findings)
	}
}

func TestTableSecurityFindings_NoFindingWithCustomerManagedKey(t *testing.T) {
	name := "orders"
	enc := "KMS"
	def := &tableDefinition{name: &name, encryptionType: &enc}

	if findings := tableSecurityFindings(def); len(findings) != 0 {
		t.Fatalf("expected no findings for a KMS-encrypted table, got %+v", findings)
	}
}
