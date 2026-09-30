package rds

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateDBList runs the same fetch as `ctl aws rds ls`, for use by the
// investigation agent (`ctl investigate`). List output has no per-resource
// evidence for the investigation's final synthesis — just a summary the
// agent can read to pick which identifier to investigate further.
func InvestigateDBList(ctx context.Context, cfg aws.Config) ([]evidence.Evidence, string, error) {
	exec := NewDBListCommandExecutor(cfg)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return nil, viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}

// InvestigateDBDef runs the same fetch as `ctl aws rds def`, returning
// Fact/Inference-tagged evidence plus a JSON summary of the same structured
// view --output json/yaml already produces. Never prints anything itself.
func InvestigateDBDef(ctx context.Context, cfg aws.Config, identifier string) ([]evidence.Evidence, string, error) {
	exec := NewDBDefinitionCommandExecutor(cfg, identifier)
	def, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return dbDefinitionEvidence(def), viewer.StructuredJSON(exec.Viewer(def, nil)), nil
}

// InvestigateDBStats runs the same fetch as `ctl aws rds stats`.
func InvestigateDBStats(ctx context.Context, cfg aws.Config, identifier string) ([]evidence.Evidence, string, error) {
	exec := NewDBStatisticsCommandExecutor(cfg, identifier)
	stats, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return dbStatisticsEvidence(stats), viewer.StructuredJSON(exec.Viewer(stats, nil)), nil
}

// InvestigateDBEvents runs the same fetch as `ctl aws rds events`.
func InvestigateDBEvents(ctx context.Context, cfg aws.Config, identifier string, since time.Duration) ([]evidence.Evidence, string, error) {
	exec := NewDBEventListCommandExecutor(cfg, identifier, since)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return dbEventListEvidence(data), viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}
