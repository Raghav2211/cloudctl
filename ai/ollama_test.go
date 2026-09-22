package ai

import (
	"cloudctl/evidence"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testFacts() []evidence.Evidence {
	return []evidence.Evidence{
		{Source: "s3:GetBucketEncryption", ResourceID: "my-bucket", Field: "Encryption", Value: "SSE-KMS", Confidence: evidence.Fact},
		{Source: "s3:GetBucketVersioning", ResourceID: "my-bucket", Field: "Versioning", Value: "Disabled", Confidence: evidence.Fact},
	}
}

func TestSummarize_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("expected request to /api/generate, got %s", r.URL.Path)
		}
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if !strings.Contains(req.Prompt, "SSE-KMS") {
			t.Errorf("expected prompt to include evidence values, got: %s", req.Prompt)
		}
		if req.Stream {
			t.Error("expected non-streaming request")
		}
		json.NewEncoder(w).Encode(generateResponse{Response: "  Versioning is disabled; deletions are unrecoverable.  "})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	got, err := client.Summarize(context.Background(), testFacts())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Versioning is disabled; deletions are unrecoverable." {
		t.Errorf("expected trimmed response, got %q", got)
	}
}

func TestSummarize_NoEvidence(t *testing.T) {
	client := NewClient("http://unused.invalid", "test-model")
	if _, err := client.Summarize(context.Background(), nil); err == nil {
		t.Fatal("expected an error when no evidence is provided, got nil")
	}
}

func TestSummarize_ServerUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := srv.URL
	srv.Close() // closed before use: nothing is listening on this address anymore

	client := NewClient(unreachableURL, "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := client.Summarize(ctx, testFacts()); err == nil {
		t.Fatal("expected an error when the Ollama server is unreachable, got nil")
	}
}

func TestSummarize_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(generateResponse{Error: "model not found"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	_, err := client.Summarize(context.Background(), testFacts())
	if err == nil {
		t.Fatal("expected an error on a non-200 response, got nil")
	}
	if !strings.Contains(err.Error(), "model not found") {
		t.Errorf("expected error to surface the ollama error message, got: %v", err)
	}
}

func TestSummarize_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(generateResponse{Response: "   "})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-model")
	if _, err := client.Summarize(context.Background(), testFacts()); err == nil {
		t.Fatal("expected an error on an empty/whitespace-only response, got nil")
	}
}

func TestSummarize_RespectsContextCancellation(t *testing.T) {
	blocked := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked // hang until the test cancels the context
	}))
	defer func() {
		close(blocked)
		srv.Close()
	}()

	client := NewClient(srv.URL, "test-model")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Summarize(ctx, testFacts())
	if err == nil {
		t.Fatal("expected a context-deadline error, got nil")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected Summarize to respect ctx cancellation quickly, took %s", elapsed)
	}
}
