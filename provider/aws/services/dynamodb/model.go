package dynamodb

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type tableSummary struct {
	name *string
}

type tableListOutput struct {
	tables []*tableSummary
}

// tableDefinition is `ctl aws dynamodb def <table>`'s output: the
// deterministic table configuration plus a Hypothesis-grade AI narration of
// it. aiSummary is additive, never a replacement for the raw fields below —
// if empty, aiSummaryUnavailable explains why (AI is never a hard
// dependency, ADR-010), mirroring bucketDefinition/instanceDefinition.
type tableDefinition struct {
	name             *string
	arn              *string
	status           *string
	itemCount        *int64
	sizeBytes        *int64
	creationDateTime *time.Time
	billingMode      *string
	gsiCount         int
	lsiCount         int
	encryptionType   *string
	streamEnabled    bool
	streamViewType   *string

	pitrStatus *string
	pitrAPIErr error

	ttlAttribute *string
	ttlStatus    *string
	ttlAPIErr    error

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

func newTableSummary(name string) *tableSummary {
	return &tableSummary{name: &name}
}

func newTableDefinition(table types.TableDescription) *tableDefinition {
	def := &tableDefinition{
		name:             table.TableName,
		arn:              table.TableArn,
		itemCount:        table.ItemCount,
		sizeBytes:        table.TableSizeBytes,
		creationDateTime: table.CreationDateTime,
		gsiCount:         len(table.GlobalSecondaryIndexes),
		lsiCount:         len(table.LocalSecondaryIndexes),
	}

	status := string(table.TableStatus)
	def.status = &status

	billingMode := "PROVISIONED"
	if table.BillingModeSummary != nil {
		billingMode = string(table.BillingModeSummary.BillingMode)
	}
	def.billingMode = &billingMode

	encryptionType := "default (DynamoDB owned key)"
	if table.SSEDescription != nil && table.SSEDescription.SSEType != "" {
		encryptionType = string(table.SSEDescription.SSEType)
	}
	def.encryptionType = &encryptionType

	if table.StreamSpecification != nil && table.StreamSpecification.StreamEnabled != nil {
		def.streamEnabled = *table.StreamSpecification.StreamEnabled
		viewType := string(table.StreamSpecification.StreamViewType)
		def.streamViewType = &viewType
	}

	return def
}

func (def *tableDefinition) SetPITR(status string) *tableDefinition {
	def.pitrStatus = &status
	return def
}

func (def *tableDefinition) SetPITRAPIError(err error) *tableDefinition {
	def.pitrAPIErr = err
	return def
}

func (def *tableDefinition) SetTTL(attribute, status string) *tableDefinition {
	def.ttlAttribute = &attribute
	def.ttlStatus = &status
	return def
}

func (def *tableDefinition) SetTTLAPIError(err error) *tableDefinition {
	def.ttlAPIErr = err
	return def
}

func (def *tableDefinition) SetAISummary(summary string) *tableDefinition {
	def.aiSummary = summary
	return def
}

func (def *tableDefinition) SetAISummaryUnavailable(reason string) *tableDefinition {
	def.aiSummaryUnavailable = reason
	return def
}

func (def *tableDefinition) SetAIRecommendations(recommendations string) *tableDefinition {
	def.aiRecommendations = recommendations
	return def
}

func (def *tableDefinition) SetAIRecommendationsUnavailable(reason string) *tableDefinition {
	def.aiRecommendationsUnavailable = reason
	return def
}
