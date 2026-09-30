package services

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"cloudctl/executor"
	"cloudctl/global"
	"cloudctl/investigate"
	"cloudctl/provider/aws"
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/changes"
	"cloudctl/provider/aws/services/dynamodb"
	"cloudctl/provider/aws/services/ec2"
	"cloudctl/provider/aws/services/eks"
	"cloudctl/provider/aws/services/lambda"
	"cloudctl/provider/aws/services/rds"
	"cloudctl/provider/aws/services/s3"
	"cloudctl/provider/aws/services/vpc"
	"cloudctl/snapshot"
	"cloudctl/viewer"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateCmd groups the investigation agent's subcommands: run a new
// investigation, or list/replay past ones from the local store (Phase 7 —
// investigations are a durable audit trail, not a one-shot terminal
// report).
type InvestigateCmd struct {
	Run  InvestigateRunCmd  `cmd:"" help:"Run a new read-only investigation"`
	List InvestigateListCmd `cmd:"" help:"List past investigations"`
	Show InvestigateShowCmd `cmd:"" help:"Replay a past investigation by ID"`
}

type InvestigateRunCmd struct {
	globals.AWSCLIFlag
	Question string `arg:"" help:"A natural-language investigation question, e.g. \"why is orders-db slow\""`
}

// Run resolves credentials once, builds the fixed AWS tool registry, and
// hands both to the bounded investigation loop (investigate.Run) — the
// roadmap's Phase 6 read-only investigation agent. Every tool the model may
// call wraps an existing, already-tested read-only command; nothing new can
// run beyond what buildAWSRegistry lists.
func (cmd *InvestigateRunCmd) Run(ctx context.Context, cli *global.CLIFlag) error {
	credentialConfig := aws.NewCredentialConfig(cmd.AWSCLIFlag, cli.Debug)
	session, err := aws.NewSessionV2(credentialConfig)
	if err != nil {
		return err
	}
	cfg := *session

	registry := buildAWSRegistry(cfg)
	exec := &executor.CommandExecutor[*investigate.Investigation]{
		Fetcher: investigationFetcher{registry: registry, question: cmd.Question},
		Viewer:  investigate.Viewer,
	}
	return exec.Execute(ctx)
}

// investigationFetcher adapts investigate.Run to the executor.Fetcher[T]
// interface, so `ctl investigate run` gets the same progress spinner,
// --output json/yaml support, and "Time elapsed" footer as every other
// command — the loop itself runs entirely inside Fetch. Persisting the
// finished investigation is a best-effort side effect here too: a local
// store failure must never hide the investigation's own result, so it's
// reported as a warning on stderr rather than returned as an error.
type investigationFetcher struct {
	registry *investigate.Registry
	question string
}

func (f investigationFetcher) Fetch(ctx context.Context) (*investigate.Investigation, error) {
	inv := investigate.Run(ctx, ai.NewClientFromEnv(), f.registry, f.question)
	if id, err := persistInvestigation(ctx, inv); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not save investigation to local history: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Saved as investigation #%d (see `ctl investigate aws show %d`)\n", id, id)
	}
	return inv, nil
}

// persistInvestigation saves a finished investigation to the local
// snapshot store, so `ctl investigate aws list`/`show` can find it later.
func persistInvestigation(ctx context.Context, inv *investigate.Investigation) (int64, error) {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return 0, err
	}
	defer store.Close()

	stepsJSON, err := json.Marshal(inv.Steps)
	if err != nil {
		return 0, fmt.Errorf("encoding steps: %w", err)
	}
	evidenceJSON, err := json.Marshal(inv.Evidence)
	if err != nil {
		return 0, fmt.Errorf("encoding evidence: %w", err)
	}

	return store.SaveInvestigation(ctx, "aws", snapshot.StoredInvestigation{
		Question:                   inv.Question,
		StoppedReason:              inv.StoppedReason,
		Conclusion:                 inv.Conclusion,
		Summary:                    inv.Summary,
		SummaryUnavailable:         inv.SummaryUnavailable,
		Recommendations:            inv.Recommendations,
		RecommendationsUnavailable: inv.RecommendationsUnavailable,
		StepsJSON:                  stepsJSON,
		EvidenceJSON:               evidenceJSON,
	})
}

type InvestigateListCmd struct {
	Limit int `default:"20" help:"Maximum number of past investigations to show"`
}

func (cmd *InvestigateListCmd) Run(ctx context.Context) error {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return err
	}
	defer store.Close()

	items, err := store.ListInvestigations(ctx, "aws", cmd.Limit)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println("No investigations saved yet. Run `ctl investigate aws run \"<question>\"` first.")
		return nil
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle("Past Investigations")
	tv.AddHeader(viewer.Row{"ID", "Question", "Stopped", "Created At"})
	for _, it := range items {
		tv.AddRow(viewer.Row{it.ID, it.Question, it.StoppedReason, it.CreatedAt})
	}
	tv.View()
	return nil
}

type InvestigateShowCmd struct {
	ID int64 `arg:"" help:"Investigation ID to replay (see 'ctl investigate aws list')"`
}

func (cmd *InvestigateShowCmd) Run(ctx context.Context) error {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return err
	}
	defer store.Close()

	stored, err := store.GetInvestigation(ctx, cmd.ID)
	if err != nil {
		return err
	}

	inv, err := investigationFromStored(stored)
	if err != nil {
		return err
	}
	investigate.Viewer(inv, nil).View()
	return nil
}

// investigationFromStored reconstructs an *investigate.Investigation from
// its persisted JSON blobs, the inverse of persistInvestigation.
func investigationFromStored(stored *snapshot.StoredInvestigation) (*investigate.Investigation, error) {
	var steps []investigate.StepRecord
	if err := json.Unmarshal(stored.StepsJSON, &steps); err != nil {
		return nil, fmt.Errorf("decoding stored steps: %w", err)
	}
	var facts []evidence.Evidence
	if err := json.Unmarshal(stored.EvidenceJSON, &facts); err != nil {
		return nil, fmt.Errorf("decoding stored evidence: %w", err)
	}

	return &investigate.Investigation{
		Question:                   stored.Question,
		Steps:                      steps,
		Evidence:                   facts,
		StoppedReason:              stored.StoppedReason,
		Conclusion:                 stored.Conclusion,
		Summary:                    stored.Summary,
		SummaryUnavailable:         stored.SummaryUnavailable,
		Recommendations:            stored.Recommendations,
		RecommendationsUnavailable: stored.RecommendationsUnavailable,
	}, nil
}

const defaultInvestigateSince = 24 * time.Hour

// sinceArg parses args[key] as a Go duration, falling back to def if the
// arg is absent or fails to parse — the agent is reminded of the expected
// format via the tool's Optional description, but a malformed value should
// degrade to a sane default rather than fail the whole step.
func sinceArg(args map[string]string, key string, def time.Duration) time.Duration {
	if raw := args[key]; raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			return d
		}
	}
	return def
}

// buildAWSRegistry lists every tool the investigation agent may call — one
// list + one def per service at minimum, plus stats/events/impact/changes
// where they exist. Deeper per-service tools (EC2 CloudWatch stats,
// security-group explain, Lambda log search) are natural follow-ups, not
// included in this first version, to keep the registry a size a local model
// can reliably choose from.
func buildAWSRegistry(cfg awssdk.Config) *investigate.Registry {
	return investigate.NewRegistry(
		investigate.Tool{
			Name:        "ec2_list",
			Description: "List EC2 instances across all states.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := ec2.InvestigateInstanceList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "ec2_def",
			Description: "Get full configuration and health for one EC2 instance.",
			Required:    []string{"instance_id"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := ec2.InvestigateInstanceDef(ctx, cfg, args["instance_id"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "rds_list",
			Description: "List RDS instances and clusters.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := rds.InvestigateDBList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "rds_def",
			Description: "Get full configuration and health for one RDS instance/cluster.",
			Required:    []string{"identifier"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := rds.InvestigateDBDef(ctx, cfg, args["identifier"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "rds_stats",
			Description: "Get CloudWatch stats (CPU, connections, storage, latency) for one RDS instance over the last 24h.",
			Required:    []string{"identifier"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := rds.InvestigateDBStats(ctx, cfg, args["identifier"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "rds_events",
			Description: "Get recent RDS lifecycle events (failovers, backups, restarts) for one instance.",
			Required:    []string{"identifier"},
			Optional:    []string{"since (a Go duration like \"24h\", default 24h)"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				since := sinceArg(args, "since", defaultInvestigateSince)
				facts, summary, err := rds.InvestigateDBEvents(ctx, cfg, args["identifier"], since)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "s3_list",
			Description: "List S3 buckets.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := s3.InvestigateBucketList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "s3_def",
			Description: "Get full configuration (policy, versioning, tags, encryption, lifecycle) for one S3 bucket.",
			Required:    []string{"bucket_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := s3.InvestigateBucketDef(ctx, cfg, args["bucket_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "s3_impact",
			Description: "Find which IAM principals/policies can access one S3 bucket.",
			Required:    []string{"bucket_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := s3.InvestigateBucketImpact(ctx, cfg, args["bucket_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "eks_list",
			Description: "List EKS clusters.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := eks.InvestigateClusterList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "eks_def",
			Description: "Get full configuration and health for one EKS cluster.",
			Required:    []string{"cluster_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := eks.InvestigateClusterDef(ctx, cfg, args["cluster_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "vpc_list",
			Description: "List VPCs.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := vpc.InvestigateVPCList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "vpc_def",
			Description: "Get full configuration and health for one VPC (subnets, routing).",
			Required:    []string{"vpc_id"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := vpc.InvestigateVPCDef(ctx, cfg, args["vpc_id"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "dynamodb_list",
			Description: "List DynamoDB tables.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := dynamodb.InvestigateTableList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "dynamodb_def",
			Description: "Get full configuration and health for one DynamoDB table.",
			Required:    []string{"table_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := dynamodb.InvestigateTableDef(ctx, cfg, args["table_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "lambda_list",
			Description: "List Lambda functions.",
			Run: func(ctx context.Context, _ map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := lambda.InvestigateFunctionList(ctx, cfg)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "lambda_def",
			Description: "Get full configuration and health for one Lambda function.",
			Required:    []string{"function_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := lambda.InvestigateFunctionDef(ctx, cfg, args["function_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "lambda_stats",
			Description: "Get CloudWatch stats (invocations, errors, throttles, duration) for one Lambda function over the last 24h.",
			Required:    []string{"function_name"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				facts, summary, err := lambda.InvestigateFunctionStats(ctx, cfg, args["function_name"])
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
		investigate.Tool{
			Name:        "changes",
			Description: "Get the recent CloudTrail change timeline, optionally filtered to one resource or event.",
			Optional:    []string{"resource", "event", "since (a Go duration like \"2h\", default 24h)"},
			Run: func(ctx context.Context, args map[string]string) (investigate.ToolResult, error) {
				since := sinceArg(args, "since", defaultInvestigateSince)
				facts, summary, err := changes.InvestigateChanges(ctx, cfg, args["resource"], args["event"], since, 50)
				return investigate.ToolResult{Evidence: facts, Summary: summary}, err
			},
		},
	)
}
