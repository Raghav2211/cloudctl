package lambda

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

// lambdaAPI is the minimal client capability this package needs, letting
// tests substitute a fake instead of a real *lambda.Client (ADR-0007).
type lambdaAPI interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
	GetFunctionConfiguration(ctx context.Context, params *lambda.GetFunctionConfigurationInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionConfigurationOutput, error)
}

type functionListFetcher struct {
	client lambdaAPI
}

type functionDefinitionFetcher struct {
	client       lambdaAPI
	functionName string
}

// Fetch lists every function, paginating via Marker/NextMarker — ListFunctions
// has no SDK-provided paginator, unlike most List* operations in this
// codebase (mirrors dynamodb's tableListFetcher.Fetch for the same reason).
func (f functionListFetcher) Fetch(ctx context.Context) (*functionListOutput, error) {
	var functions []*functionSummary
	var marker *string
	for {
		out, err := f.client.ListFunctions(ctx, &lambda.ListFunctionsInput{Marker: marker})
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, fn := range out.Functions {
			functions = append(functions, newFunctionSummary(fn.FunctionName, string(fn.Runtime), string(fn.State)))
		}
		if out.NextMarker == nil {
			break
		}
		marker = out.NextMarker
	}
	if len(functions) == 0 {
		return nil, ctlaws.NewErrorInfo(NoFunctionFound(), viewer.INFO, nil)
	}
	return &functionListOutput{functions: functions}, nil
}

// Fetch retrieves a function's configuration and narrates it via
// Summarize/Recommend, mirroring tableDefinitionFetcher.Fetch (dynamodb).
func (f functionDefinitionFetcher) Fetch(ctx context.Context) (*functionDefinition, error) {
	out, err := f.client.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{FunctionName: &f.functionName})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	def := newFunctionDefinition(out)
	def.applyAINarration(ctx, ai.NewClientFromEnv(), functionDefinitionEvidence(def))
	return def, nil
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks) from the given evidence, never returning an
// error (ADR-0010). Summarize and Recommend run concurrently under a single
// spinner rather than back-to-back, mirroring dynamodb's applyAINarration.
func (def *functionDefinition) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		def.SetAISummaryUnavailable("no evidence could be gathered")
		def.SetAIRecommendationsUnavailable("no evidence could be gathered")
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		def.SetAISummaryUnavailable(result.summaryErr)
	} else {
		def.SetAISummary(result.summary)
	}
	if result.recommendedErr != "" {
		def.SetAIRecommendationsUnavailable(result.recommendedErr)
	} else {
		def.SetAIRecommendations(result.recommendations)
	}
}
