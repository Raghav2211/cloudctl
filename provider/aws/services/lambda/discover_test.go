package lambda

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

func TestDiscover_HappyPath(t *testing.T) {
	client := &fakeLambdaClient{
		listPages: []*lambda.ListFunctionsOutput{
			{Functions: []types.FunctionConfiguration{
				{FunctionName: aws.String("orders-handler")},
				{FunctionName: aws.String("users-handler")},
			}},
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
		if r.Type != "lambda:function" {
			t.Errorf("expected type lambda:function, got %q", r.Type)
		}
	}
}

func TestDiscover_EmptyIsNotAnError(t *testing.T) {
	client := &fakeLambdaClient{listPages: []*lambda.ListFunctionsOutput{{}}}

	resources, err := Discover(context.Background(), client)
	if err != nil {
		t.Fatalf("expected zero functions to not be an error for discovery, got %v", err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected 0 resources, got %d", len(resources))
	}
}

func TestDiscover_APIError(t *testing.T) {
	client := &fakeLambdaClient{listErr: errors.New("boom")}

	_, err := Discover(context.Background(), client)
	if err == nil {
		t.Fatal("expected an error from a failing ListFunctions call, got nil")
	}
}
