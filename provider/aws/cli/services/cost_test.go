package services

import (
	"cloudctl/cost"
	"errors"
	"testing"
)

func TestAggregateIdleScanResults_SkipsFailedServicesButKeepsSucceeded(t *testing.T) {
	results := []idleScanResult{
		{findings: []cost.Finding{{Rule: "ec2-idle-low-cpu", ResourceID: "i-abc"}}, scanned: 3},
		{err: errors.New("rds: access denied")},
		{scanned: 5},
	}

	findings, scanned, err := aggregateIdleScanResults(results)
	if err != nil {
		t.Fatalf("expected no error when at least one service succeeded, got %v", err)
	}
	if len(findings) != 1 || findings[0].ResourceID != "i-abc" {
		t.Fatalf("expected the succeeded service's finding to survive, got %+v", findings)
	}
	if scanned != 8 {
		t.Fatalf("expected scanned=8 (3+5, skipping the failed service), got %d", scanned)
	}
}

func TestAggregateIdleScanResults_ReturnsErrorWhenEveryServiceFails(t *testing.T) {
	wantErr := errors.New("ec2: access denied")
	results := []idleScanResult{
		{err: wantErr},
		{err: errors.New("rds: access denied")},
		{err: errors.New("lambda: access denied")},
	}

	_, _, err := aggregateIdleScanResults(results)
	if err != wantErr {
		t.Fatalf("expected the first service's error to be surfaced when all fail, got %v", err)
	}
}
