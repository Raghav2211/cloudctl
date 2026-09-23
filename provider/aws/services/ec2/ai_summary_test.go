package ec2

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"context"
	"net"
	"testing"
	"time"
)

// TestApplyAINarration_NoEvidence_SetsUnavailable mirrors the s3 package's
// equivalent test: if no evidence could be gathered, applyAINarration must
// never even attempt a Summarize/Recommend call.
func TestApplyAINarration_NoEvidence_SetsUnavailable(t *testing.T) {
	def := newInstanceDefinition()
	def.applyAINarration(context.Background(), ai.NewClient("http://127.0.0.1:1", "unused"), nil)

	if def.aiSummary != "" {
		t.Fatalf("expected no summary, got %q", def.aiSummary)
	}
	if def.aiSummaryUnavailable == "" {
		t.Fatal("expected aiSummaryUnavailable to be set")
	}
	if def.aiRecommendations != "" {
		t.Fatalf("expected no recommendations, got %q", def.aiRecommendations)
	}
	if def.aiRecommendationsUnavailable == "" {
		t.Fatal("expected aiRecommendationsUnavailable to be set")
	}
}

// TestApplyAINarration_LLMUnreachable_GracefulFallback is the ADR-010
// verification for the ec2 def narrator: point at a real, closed TCP port
// (guaranteed unreachable, not a mock) and confirm instanceDefinition ends up
// in a valid, renderable state instead of Fetch failing outright.
func TestApplyAINarration_LLMUnreachable_GracefulFallback(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	addr := "http://" + l.Addr().String()
	l.Close()

	def := newInstanceDefinition()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	facts := []evidence.Evidence{
		{Source: "ec2:DescribeInstances", ResourceID: "i-0123456789abcdef0", Field: "InstanceType", Value: "t3.micro", Confidence: evidence.Fact},
	}
	def.applyAINarration(ctx, ai.NewClient(addr, "unused"), facts)

	if def.aiSummary != "" {
		t.Fatalf("expected no summary when the LLM backend is unreachable, got %q", def.aiSummary)
	}
	if def.aiSummaryUnavailable == "" {
		t.Fatal("expected aiSummaryUnavailable to explain the failure")
	}
	if def.aiRecommendations != "" {
		t.Fatalf("expected no recommendations when the LLM backend is unreachable, got %q", def.aiRecommendations)
	}
	if def.aiRecommendationsUnavailable == "" {
		t.Fatal("expected aiRecommendationsUnavailable to explain the failure")
	}
	t.Logf("graceful fallback message: summary=%s recommendations=%s", def.aiSummaryUnavailable, def.aiRecommendationsUnavailable)
}
