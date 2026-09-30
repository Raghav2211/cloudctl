package lambda

import (
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/security"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// NewFunctionSecurityCommandExecutor runs `ctl aws lambda security <name>`:
// the same definition fetch as `ctl aws lambda def`, evaluated against a
// small set of deterministic security rules (never AI-generated — see
// cloudctl/security's package doc).
func NewFunctionSecurityCommandExecutor(cfg aws.Config, functionName string) *executor.CommandExecutor[*security.Report] {
	return &executor.CommandExecutor[*security.Report]{
		Fetcher: functionSecurityFetcher{cfg: cfg, functionName: functionName},
		Viewer:  security.Viewer,
	}
}

type functionSecurityFetcher struct {
	cfg          aws.Config
	functionName string
}

func (f functionSecurityFetcher) Fetch(ctx context.Context) (*security.Report, error) {
	def, err := NewFunctionDefinitionCommandExecutor(f.cfg, f.functionName).Fetcher.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	return &security.Report{ResourceID: f.functionName, Findings: functionSecurityFindings(def)}, nil
}

const ruleLambdaDeprecatedRuntime = "lambda-deprecated-runtime"

// deprecatedRuntimes lists Lambda runtime identifiers AWS has placed in
// deprecation/blocked status as of early 2026 — a function stuck on one of
// these no longer receives security patches for its language runtime, and
// AWS eventually blocks updates/invokes on fully unsupported ones. This list
// needs periodic updates as AWS deprecates further runtimes over time; it
// is deliberately conservative (only long-since-EOL runtimes), to avoid
// flagging a runtime that's merely old but still supported.
var deprecatedRuntimes = map[string]bool{
	"python2.7":     true,
	"python3.6":     true,
	"python3.7":     true,
	"python3.8":     true,
	"nodejs10.x":    true,
	"nodejs12.x":    true,
	"dotnetcore2.1": true,
	"dotnetcore3.1": true,
	"ruby2.5":       true,
	"ruby2.7":       true,
}

// functionSecurityFindings flags a function running on a deprecated
// runtime — directly readable from the same GetFunctionConfiguration data
// `ctl aws lambda def` already fetches.
func functionSecurityFindings(def *functionDefinition) []security.Finding {
	if def == nil || !deprecatedRuntimes[def.runtime] {
		return nil
	}
	id := derefStr(def.name)
	return []security.Finding{{
		Rule:        ruleLambdaDeprecatedRuntime,
		Severity:    security.Medium,
		ResourceID:  id,
		Description: "This function runs on runtime " + def.runtime + ", which AWS has deprecated and no longer patches for security issues.",
		Remediation: "Migrate the function to a currently-supported runtime version.",
		Evidence: []evidence.Evidence{
			{Source: "lambda:GetFunctionConfiguration", ResourceID: id, Field: "Runtime", Value: def.runtime, Confidence: evidence.Fact},
		},
	}}
}
