package dynamodb

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// dynamoDBAPI is the minimal client capability this package needs, letting
// tests substitute a fake instead of a real *dynamodb.Client (ADR-007).
type dynamoDBAPI interface {
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error)
	DescribeTimeToLive(ctx context.Context, params *dynamodb.DescribeTimeToLiveInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error)
}

type tableListFetcher struct {
	client dynamoDBAPI
}

type tableDefinitionFetcher struct {
	client    dynamoDBAPI
	tableName string
}

// Fetch lists every table name, paginating via ExclusiveStartTableName —
// ListTables has no SDK-provided paginator, unlike most List* operations in
// this codebase.
func (f tableListFetcher) Fetch(ctx context.Context) (*tableListOutput, error) {
	var names []string
	var startTable *string
	for {
		out, err := f.client.ListTables(ctx, &dynamodb.ListTablesInput{ExclusiveStartTableName: startTable})
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		names = append(names, out.TableNames...)
		if out.LastEvaluatedTableName == nil {
			break
		}
		startTable = out.LastEvaluatedTableName
	}
	if len(names) == 0 {
		return nil, ctlaws.NewErrorInfo(NoTableFound(), viewer.INFO, nil)
	}

	tables := make([]*tableSummary, 0, len(names))
	for _, name := range names {
		tables = append(tables, newTableSummary(name))
	}
	return &tableListOutput{tables: tables}, nil
}

// Fetch retrieves a table's configuration and narrates it via Summarize,
// mirroring bucketConfigurationFetcher.Fetch (s3) and
// instanceDefinitionFetcher.Fetch (ec2). PITR and TTL are each a separate
// API call outside DescribeTable itself; a failure in either is recorded on
// the definition rather than failing the whole command (ADR-010's
// AI-narration-is-additive discipline extended to these sub-fetches too).
func (f tableDefinitionFetcher) Fetch(ctx context.Context) (*tableDefinition, error) {
	descOut, err := f.client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: &f.tableName})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if descOut.Table == nil {
		return nil, ctlaws.NewErrorInfo(TableNotFound(f.tableName), viewer.INFO, nil)
	}

	def := newTableDefinition(*descOut.Table)

	if backupsOut, err := f.client.DescribeContinuousBackups(ctx, &dynamodb.DescribeContinuousBackupsInput{TableName: &f.tableName}); err != nil {
		def.SetPITRAPIError(ctlaws.AWSError(err))
	} else if backupsOut.ContinuousBackupsDescription != nil && backupsOut.ContinuousBackupsDescription.PointInTimeRecoveryDescription != nil {
		def.SetPITR(string(backupsOut.ContinuousBackupsDescription.PointInTimeRecoveryDescription.PointInTimeRecoveryStatus))
	}

	if ttlOut, err := f.client.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{TableName: &f.tableName}); err != nil {
		def.SetTTLAPIError(ctlaws.AWSError(err))
	} else if ttlOut.TimeToLiveDescription != nil {
		def.SetTTL(derefStr(ttlOut.TimeToLiveDescription.AttributeName), string(ttlOut.TimeToLiveDescription.TimeToLiveStatus))
	}

	facts := tableDefinitionEvidence(def)
	def.applyAISummary(ctx, ai.NewClientFromEnv(), facts)

	return def, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error (ADR-010). Mirrors every other
// applyAISummary in this codebase (s3, ec2).
func (def *tableDefinition) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		def.SetAISummaryUnavailable("no evidence could be gathered")
		return
	}
	summary, err := viewer.WithSpinner("Generating AI summary...", func() (string, error) {
		return client.Summarize(ctx, facts)
	})
	if err != nil {
		def.SetAISummaryUnavailable(err.Error())
		return
	}
	def.SetAISummary(summary)
}
