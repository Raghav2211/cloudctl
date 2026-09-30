package lambda

import "github.com/aws/aws-sdk-go-v2/service/lambda"

type functionSummary struct {
	name    *string
	runtime string
	state   string
}

type functionListOutput struct {
	functions []*functionSummary
}

// functionDefinition is `ctl aws lambda def <name>`'s output: the
// deterministic function configuration plus a Hypothesis-grade AI narration
// of it. aiSummary is additive, never a replacement for the raw fields below
// — if empty, aiSummaryUnavailable explains why (ADR-0010), mirroring every
// other *Definition type in this codebase.
type functionDefinition struct {
	name          *string
	arn           *string
	runtime       string
	handler       *string
	role          *string
	memorySizeMB  *int32
	timeoutSec    *int32
	codeSizeBytes int64
	lastModified  *string
	state         string
	packageType   string
	architectures []string

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

func newFunctionSummary(name *string, runtime, state string) *functionSummary {
	return &functionSummary{name: name, runtime: runtime, state: state}
}

func newFunctionDefinition(out *lambda.GetFunctionConfigurationOutput) *functionDefinition {
	architectures := make([]string, 0, len(out.Architectures))
	for _, a := range out.Architectures {
		architectures = append(architectures, string(a))
	}
	return &functionDefinition{
		name:          out.FunctionName,
		arn:           out.FunctionArn,
		runtime:       string(out.Runtime),
		handler:       out.Handler,
		role:          out.Role,
		memorySizeMB:  out.MemorySize,
		timeoutSec:    out.Timeout,
		codeSizeBytes: out.CodeSize,
		lastModified:  out.LastModified,
		state:         string(out.State),
		packageType:   string(out.PackageType),
		architectures: architectures,
	}
}

func (def *functionDefinition) SetAISummary(summary string) *functionDefinition {
	def.aiSummary = summary
	return def
}

func (def *functionDefinition) SetAISummaryUnavailable(reason string) *functionDefinition {
	def.aiSummaryUnavailable = reason
	return def
}

func (def *functionDefinition) SetAIRecommendations(recommendations string) *functionDefinition {
	def.aiRecommendations = recommendations
	return def
}

func (def *functionDefinition) SetAIRecommendationsUnavailable(reason string) *functionDefinition {
	def.aiRecommendationsUnavailable = reason
	return def
}
