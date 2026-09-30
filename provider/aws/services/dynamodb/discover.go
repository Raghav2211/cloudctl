package dynamodb

import (
	"cloudctl/snapshot"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Discover fetches every DynamoDB table name (unfiltered), for `ctl discover
// aws`. ListTables returns only names, not full table configuration —
// matching s3's Discover, which stores the same lightweight bucket-list
// detail rather than an extra describe call per resource; use `ctl aws
// dynamodb def <table>` for full configuration. Calls the client directly
// rather than going through tableListFetcher.Fetch, which turns "zero
// tables" into a business error (NoTableFound) — a legitimate, non-fatal
// outcome for discovery.
func Discover(ctx context.Context, client dynamoDBAPI) ([]snapshot.Resource, error) {
	var resources []snapshot.Resource
	var startTable *string
	for {
		out, err := client.ListTables(ctx, &dynamodb.ListTablesInput{ExclusiveStartTableName: startTable})
		if err != nil {
			return nil, err
		}
		for _, name := range out.TableNames {
			resources = append(resources, snapshot.Resource{ID: name, Type: "dynamodb:table", Attrs: name})
		}
		if out.LastEvaluatedTableName == nil {
			break
		}
		startTable = out.LastEvaluatedTableName
	}
	return resources, nil
}
