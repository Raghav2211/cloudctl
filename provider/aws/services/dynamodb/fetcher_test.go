package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDynamoDBClient implements dynamoDBAPI with canned responses, letting
// tests substitute it for a real *dynamodb.Client (ADR-007).
type fakeDynamoDBClient struct {
	listPages   []*dynamodb.ListTablesOutput
	listCalls   int
	listErr     error
	describeOut *dynamodb.DescribeTableOutput
	describeErr error
	backupsOut  *dynamodb.DescribeContinuousBackupsOutput
	backupsErr  error
	ttlOut      *dynamodb.DescribeTimeToLiveOutput
	ttlErr      error
}

func (f *fakeDynamoDBClient) ListTables(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listCalls >= len(f.listPages) {
		return &dynamodb.ListTablesOutput{}, nil
	}
	page := f.listPages[f.listCalls]
	f.listCalls++
	return page, nil
}

func (f *fakeDynamoDBClient) DescribeTable(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	return f.describeOut, nil
}

func (f *fakeDynamoDBClient) DescribeContinuousBackups(_ context.Context, _ *dynamodb.DescribeContinuousBackupsInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error) {
	if f.backupsErr != nil {
		return nil, f.backupsErr
	}
	return f.backupsOut, nil
}

func (f *fakeDynamoDBClient) DescribeTimeToLive(_ context.Context, _ *dynamodb.DescribeTimeToLiveInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
	if f.ttlErr != nil {
		return nil, f.ttlErr
	}
	return f.ttlOut, nil
}

func TestTableListFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeDynamoDBClient{
		listPages: []*dynamodb.ListTablesOutput{
			{TableNames: []string{"orders", "users"}},
		},
	}
	f := tableListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(out.tables))
	}
}

func TestTableListFetcher_Fetch_PaginatesAcrossMultiplePages(t *testing.T) {
	nextToken := "users"
	client := &fakeDynamoDBClient{
		listPages: []*dynamodb.ListTablesOutput{
			{TableNames: []string{"orders"}, LastEvaluatedTableName: &nextToken},
			{TableNames: []string{"users"}},
		},
	}
	f := tableListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.tables) != 2 {
		t.Fatalf("expected 2 tables across 2 pages, got %d", len(out.tables))
	}
	if client.listCalls != 2 {
		t.Fatalf("expected the paginator to stop after 2 calls, made %d", client.listCalls)
	}
}

func TestTableListFetcher_Fetch_EmptyResult(t *testing.T) {
	client := &fakeDynamoDBClient{listPages: []*dynamodb.ListTablesOutput{{}}}
	f := tableListFetcher{client: client}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no tables are found, got nil")
	}
}

func TestTableListFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeDynamoDBClient{listErr: errors.New("boom")}
	f := tableListFetcher{client: client}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing ListTables call, got nil")
	}
}

func testTableDescription() types.TableDescription {
	name := "orders"
	arn := "arn:aws:dynamodb:eu-west-1:123456789012:table/orders"
	return types.TableDescription{
		TableName:   &name,
		TableArn:    &arn,
		TableStatus: types.TableStatusActive,
		BillingModeSummary: &types.BillingModeSummary{
			BillingMode: types.BillingModePayPerRequest,
		},
		SSEDescription: &types.SSEDescription{
			SSEType: types.SSETypeKms,
		},
		StreamSpecification: &types.StreamSpecification{
			StreamEnabled:  aws.Bool(true),
			StreamViewType: types.StreamViewTypeNewAndOldImages,
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndexDescription{{}},
	}
}

func TestTableDefinitionFetcher_Fetch_EndToEnd(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	client := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: ptr(testTableDescription())},
		backupsOut: &dynamodb.DescribeContinuousBackupsOutput{
			ContinuousBackupsDescription: &types.ContinuousBackupsDescription{
				PointInTimeRecoveryDescription: &types.PointInTimeRecoveryDescription{
					PointInTimeRecoveryStatus: types.PointInTimeRecoveryStatusEnabled,
				},
			},
		},
		ttlOut: &dynamodb.DescribeTimeToLiveOutput{
			TimeToLiveDescription: &types.TimeToLiveDescription{
				AttributeName:    aws.String("expiresAt"),
				TimeToLiveStatus: types.TimeToLiveStatusEnabled,
			},
		},
	}
	f := tableDefinitionFetcher{client: client, tableName: "orders"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if derefStr(def.name) != "orders" {
		t.Errorf("expected name 'orders', got %q", derefStr(def.name))
	}
	if def.gsiCount != 1 {
		t.Errorf("expected 1 GSI, got %d", def.gsiCount)
	}
	if !def.streamEnabled {
		t.Error("expected stream to be enabled")
	}
	if derefStr(def.pitrStatus) != "ENABLED" {
		t.Errorf("expected PITR ENABLED, got %q", derefStr(def.pitrStatus))
	}
	if derefStr(def.ttlStatus) != "ENABLED" {
		t.Errorf("expected TTL ENABLED, got %q", derefStr(def.ttlStatus))
	}
	if def.aiSummary != "" {
		t.Errorf("expected no AI summary with Ollama unreachable, got %q", def.aiSummary)
	}
	if def.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to explain why the summary is missing")
	}

	// Must render without panicking regardless of AI availability.
	tableDefinitionViewer(def, nil).View()
}

// TestTableDefinitionFetcher_Fetch_DegradesGracefullyOnPITRTTLErrors
// confirms that a real table's DescribeTable succeeding while
// DescribeContinuousBackups/DescribeTimeToLive independently fail (e.g.
// permission denied for one but not the other) still produces a fully
// renderable definition — mirroring the partial-failure tolerance already
// established for s3's bucketConfigurationFetcher.
func TestTableDefinitionFetcher_Fetch_DegradesGracefullyOnPITRTTLErrors(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	client := &fakeDynamoDBClient{
		describeOut: &dynamodb.DescribeTableOutput{Table: ptr(testTableDescription())},
		backupsErr:  errors.New("access denied"),
		ttlErr:      errors.New("access denied"),
	}
	f := tableDefinitionFetcher{client: client, tableName: "orders"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected PITR/TTL failures not to fail the whole fetch: %v", err)
	}
	if def.pitrAPIErr == nil {
		t.Error("expected pitrAPIErr to be set")
	}
	if def.ttlAPIErr == nil {
		t.Error("expected ttlAPIErr to be set")
	}

	tableDefinitionViewer(def, nil).View() // must not panic
}

func TestTableDefinitionFetcher_Fetch_NotFound(t *testing.T) {
	client := &fakeDynamoDBClient{describeOut: &dynamodb.DescribeTableOutput{}}
	f := tableDefinitionFetcher{client: client, tableName: "does-not-exist"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when the table doesn't exist, got nil")
	}
}

func TestTableDefinitionFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeDynamoDBClient{describeErr: errors.New("boom")}
	f := tableDefinitionFetcher{client: client, tableName: "orders"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing DescribeTable call, got nil")
	}
}

func ptr[T any](v T) *T { return &v }
