package dynamodb

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func TestDiscover_HappyPath(t *testing.T) {
	client := &fakeDynamoDBClient{
		listPages: []*dynamodb.ListTablesOutput{
			{TableNames: []string{"orders", "users"}},
		},
	}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	for _, r := range resources {
		if r.Type != "dynamodb:table" {
			t.Errorf("expected type dynamodb:table, got %q", r.Type)
		}
	}
}

func TestDiscover_EmptyIsNotAnError(t *testing.T) {
	client := &fakeDynamoDBClient{listPages: []*dynamodb.ListTablesOutput{{}}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("expected zero tables to not be an error for discovery, got %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestDiscover_APIError(t *testing.T) {
	client := &fakeDynamoDBClient{listErr: errors.New("boom")}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing ListTables call, got nil")
	}
}
