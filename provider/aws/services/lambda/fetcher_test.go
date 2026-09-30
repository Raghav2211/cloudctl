package lambda

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
)

// fakeLambdaClient implements lambdaAPI with canned responses, letting
// tests substitute it for a real *lambda.Client (ADR-0007).
type fakeLambdaClient struct {
	listPages []*lambda.ListFunctionsOutput
	listCalls int
	listErr   error

	getOut *lambda.GetFunctionConfigurationOutput
	getErr error
}

func (f *fakeLambdaClient) ListFunctions(_ context.Context, _ *lambda.ListFunctionsInput, _ ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listCalls >= len(f.listPages) {
		return &lambda.ListFunctionsOutput{}, nil
	}
	page := f.listPages[f.listCalls]
	f.listCalls++
	return page, nil
}

func (f *fakeLambdaClient) GetFunctionConfiguration(_ context.Context, _ *lambda.GetFunctionConfigurationInput, _ ...func(*lambda.Options)) (*lambda.GetFunctionConfigurationOutput, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getOut, nil
}

func TestFunctionListFetcher_Fetch_HappyPath(t *testing.T) {
	client := &fakeLambdaClient{
		listPages: []*lambda.ListFunctionsOutput{
			{Functions: []types.FunctionConfiguration{
				{FunctionName: aws.String("orders-handler"), Runtime: types.RuntimePython313, State: types.StateActive},
				{FunctionName: aws.String("users-handler"), Runtime: types.RuntimeNodejs20x, State: types.StateActive},
			}},
		},
	}
	f := functionListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.functions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(out.functions))
	}
}

func TestFunctionListFetcher_Fetch_PaginatesAcrossMultiplePages(t *testing.T) {
	nextMarker := "page2"
	client := &fakeLambdaClient{
		listPages: []*lambda.ListFunctionsOutput{
			{Functions: []types.FunctionConfiguration{{FunctionName: aws.String("orders-handler")}}, NextMarker: &nextMarker},
			{Functions: []types.FunctionConfiguration{{FunctionName: aws.String("users-handler")}}},
		},
	}
	f := functionListFetcher{client: client}

	out, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.functions) != 2 {
		t.Fatalf("expected 2 functions across 2 pages, got %d", len(out.functions))
	}
	if client.listCalls != 2 {
		t.Fatalf("expected the paginator to stop after 2 calls, made %d", client.listCalls)
	}
}

func TestFunctionListFetcher_Fetch_EmptyResult(t *testing.T) {
	client := &fakeLambdaClient{listPages: []*lambda.ListFunctionsOutput{{}}}
	f := functionListFetcher{client: client}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error when no functions are found, got nil")
	}
}

func TestFunctionListFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeLambdaClient{listErr: errors.New("boom")}
	f := functionListFetcher{client: client}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing ListFunctions call, got nil")
	}
}

func TestFunctionDefinitionFetcher_Fetch_EndToEnd(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1") // fast, deterministic AI-unavailable path

	client := &fakeLambdaClient{
		getOut: &lambda.GetFunctionConfigurationOutput{
			FunctionName: aws.String("orders-handler"),
			FunctionArn:  aws.String("arn:aws:lambda:eu-west-1:123456789012:function:orders-handler"),
			Runtime:      types.RuntimePython313,
			Handler:      aws.String("main.handler"),
			Role:         aws.String("arn:aws:iam::123456789012:role/orders-handler-role"),
			MemorySize:   aws.Int32(256),
			Timeout:      aws.Int32(30),
			CodeSize:     1024,
			State:        types.StateActive,
			PackageType:  types.PackageTypeZip,
		},
	}
	f := functionDefinitionFetcher{client: client, functionName: "orders-handler"}

	def, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if derefStr(def.name) != "orders-handler" {
		t.Errorf("expected name 'orders-handler', got %q", derefStr(def.name))
	}
	if def.runtime != "python3.13" {
		t.Errorf("expected runtime 'python3.13', got %q", def.runtime)
	}
	if def.aiSummary != "" {
		t.Errorf("expected no AI summary with Ollama unreachable, got %q", def.aiSummary)
	}
	if def.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to explain why the summary is missing")
	}
	if def.aiRecommendations != "" {
		t.Errorf("expected no AI recommendations with Ollama unreachable, got %q", def.aiRecommendations)
	}
	if def.aiRecommendationsUnavailable == "" {
		t.Error("expected aiRecommendationsUnavailable to explain why recommendations are missing")
	}

	// Must render without panicking regardless of AI availability.
	functionDefinitionViewer(def, nil).View()
}

func TestFunctionDefinitionFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeLambdaClient{getErr: errors.New("boom")}
	f := functionDefinitionFetcher{client: client, functionName: "does-not-exist"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing GetFunctionConfiguration call, got nil")
	}
}
