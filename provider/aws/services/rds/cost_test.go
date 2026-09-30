package rds

import "testing"

func TestDBIdleFindings_FlagsLowUtilization(t *testing.T) {
	cpu, conns := 1.0, 0.0
	stats := &dbStatistics{identifier: "orders-db", cpuPercent: &cpu, connectionsAvg: &conns}

	findings := dbIdleFindings(stats)
	if len(findings) != 1 || findings[0].Rule != ruleDBIdleLowUtilization {
		t.Fatalf("expected 1 %s finding, got %+v", ruleDBIdleLowUtilization, findings)
	}
}

func TestDBIdleFindings_NoFindingForActiveInstance(t *testing.T) {
	cpu, conns := 40.0, 12.0
	stats := &dbStatistics{identifier: "orders-db", cpuPercent: &cpu, connectionsAvg: &conns}

	if findings := dbIdleFindings(stats); len(findings) != 0 {
		t.Fatalf("expected no findings for an active instance, got %+v", findings)
	}
}

func TestDBIdleFindings_NoFindingWhenDataMissing(t *testing.T) {
	if findings := dbIdleFindings(&dbStatistics{identifier: "orders-db"}); len(findings) != 0 {
		t.Fatalf("expected no findings when metrics are unavailable, got %+v", findings)
	}
}
