package dynamodb

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// defaultOwnedKeyEncryptionType is the encryptionType value newTableDefinition
// sets when a table has no SSEDescription — i.e. it relies on DynamoDB's
// default AWS-owned encryption key rather than a customer-managed KMS key.
// Shared between model.go (where it's set) and security.go (where it's
// checked), so the two can't silently drift apart.
const defaultOwnedKeyEncryptionType = "default (DynamoDB owned key)"

// NewTableSecurityCommandExecutor runs `ctl aws dynamodb security <table>`:
// the same definition fetch as `ctl aws dynamodb def`, evaluated against a
// small set of deterministic security rules (never AI-generated — see
// cloudctl/security's package doc).
func NewTableSecurityCommandExecutor(cfg aws.Config, tableName string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: tableSecurityFetcher{cfg: cfg, tableName: tableName},
		Viewer:  security.Viewer,
	}
}

type tableSecurityFetcher struct {
	cfg       aws.Config
	tableName string
}

func (f tableSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewTableDefinitionCommandExecutor(f.cfg, f.tableName).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.tableName, Findings: tableSecurityFindings(def)}, nil
}

const ruleDynamoDBDefaultEncryptionKey = "dynamodb-default-encryption-key"

// tableSecurityFindings flags a table relying on DynamoDB's default
// AWS-owned encryption key rather than a customer-managed KMS key — a
// customer-managed key gives independent audit trail, rotation control,
// and the ability to revoke access, none of which the default key offers.
// Directly readable from the same DescribeTable data `ctl aws dynamodb def`
// already fetches.
func tableSecurityFindings(def *tableDefinition) []security.Finding {
	if def == nil || def.encryptionType == nil || *def.encryptionType != defaultOwnedKeyEncryptionType {
		return nil
	}
	id := derefStr(def.name)
	return []security.Finding{{
		Rule:        ruleDynamoDBDefaultEncryptionKey,
		Severity:    security.Medium,
		ResourceID:  id,
		Description: "This table is encrypted with DynamoDB's default AWS-owned key rather than a customer-managed KMS key.",
		Remediation: "Enable server-side encryption with a customer-managed KMS key for independent audit trail and key rotation/revocation control.",
		Evidence: []evidence.Evidence{
			{Source: "dynamodb:DescribeTable", ResourceID: id, Field: "Encryption", Value: *def.encryptionType, Confidence: evidence.Fact},
		},
	}}
}
