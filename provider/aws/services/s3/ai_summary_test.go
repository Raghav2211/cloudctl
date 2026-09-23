package s3

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func testEvidence() []evidence.Evidence {
	return []evidence.Evidence{
		{Source: "s3:GetBucketEncryption", ResourceID: "my-bucket", Field: "Encryption", Value: "AES256", Confidence: evidence.Fact},
		{Source: "s3:GetBucketVersioning", ResourceID: "my-bucket", Field: "Versioning", Value: "Disabled", Confidence: evidence.Fact},
	}
}

// TestApplyAINarration_NoEvidence_SetsUnavailable exercises the graceful
// fallback path with no live dependency at all: if every dimension's fetch
// failed, applyAINarration must never even attempt a Summarize/Recommend call.
func TestApplyAINarration_NoEvidence_SetsUnavailable(t *testing.T) {
	def := &bucketDefinition{}
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
// verification: point at a real, closed TCP port (guaranteed unreachable,
// not a mock) and confirm bucketDefinition ends up in a valid, renderable
// state instead of Fetch failing outright.
func TestApplyAINarration_LLMUnreachable_GracefulFallback(t *testing.T) {
	// Reserve a port and immediately close it: it existed a moment ago and
	// is now guaranteed to refuse connections.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a port: %v", err)
	}
	addr := "http://" + l.Addr().String()
	l.Close()

	def := &bucketDefinition{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	def.applyAINarration(ctx, ai.NewClient(addr, "unused"), testEvidence())

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

	// The raw data is still fully renderable even though AI failed — this
	// is the whole point of ADR-010 (additive, never a hard dependency).
	// Every field is left nil here (as if every dimension's fetch also
	// failed) to confirm the render path is nil-safe, not just AI-failure-safe.
	def.SetBucketName("my-bucket")
	view := bucketConfigurationViewer(def, nil)
	view.View() // must not panic even though every field is nil
}

// TestApplyAINarration_RealOllama_LivePath is a real (not mocked) end-to-end
// check against a local Ollama server, matching how ctl s3 def actually
// calls Summarize/Recommend. It skips itself if OLLAMA_MODEL isn't set,
// since CI/other machines won't have a model pulled.
func TestApplyAINarration_RealOllama_LivePath(t *testing.T) {
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		t.Skip("OLLAMA_MODEL not set; skipping live Ollama check (see ai package tests for the mocked equivalent)")
	}

	def := &bucketDefinition{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	def.applyAINarration(ctx, ai.NewClientFromEnv(), testEvidence())

	if def.aiSummaryUnavailable != "" {
		t.Fatalf("expected a live Ollama call to succeed, got unavailable: %s", def.aiSummaryUnavailable)
	}
	if def.aiSummary == "" {
		t.Fatal("expected a non-empty summary from the live Ollama call")
	}
	if !strings.Contains(strings.ToUpper(def.aiSummary), "AES") {
		t.Logf("summary did not literally mention AES256 (model paraphrased): %s", def.aiSummary)
	}
	t.Logf("live summary: %s", def.aiSummary)

	if def.aiRecommendationsUnavailable != "" {
		t.Fatalf("expected a live Ollama Recommend call to succeed, got unavailable: %s", def.aiRecommendationsUnavailable)
	}
	if def.aiRecommendations == "" {
		t.Fatal("expected non-empty recommendations from the live Ollama call")
	}
	t.Logf("live recommendations: %s", def.aiRecommendations)
}
