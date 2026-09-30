package investigate

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
}

// scriptedOllama serves one canned /api/generate response per call, in
// order, letting tests drive the agent loop through a specific sequence of
// model decisions without a real Ollama server.
func scriptedOllama(t *testing.T, responses []string) *httptest.Server {
	t.Helper()
	var calls int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		i := atomic.AddInt32(&calls, 1) - 1
		if int(i) >= len(responses) {
			t.Fatalf("unexpected extra call to ollama (call #%d), only %d scripted", i+1, len(responses))
		}
		json.NewEncoder(w).Encode(generateResponse{Response: responses[i]})
	}))
}

func echoTool(name string) Tool {
	return Tool{
		Name:        name,
		Description: "a test tool",
		Required:    []string{"id"},
		Run: func(_ context.Context, args map[string]string) (ToolResult, error) {
			return ToolResult{
				Evidence: []evidence.Evidence{{Source: "test", ResourceID: args["id"], Field: "Field", Value: "Value", Confidence: evidence.Fact}},
				Summary:  fmt.Sprintf("%s called with id=%s", name, args["id"]),
			}, nil
		},
	}
}

func TestRun_ConcludesAfterOneToolCall(t *testing.T) {
	var actCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateRequest
		json.NewDecoder(r.Body).Decode(&req)
		switch {
		case strings.Contains(req.Prompt, "You are summarizing"):
			json.NewEncoder(w).Encode(generateResponse{Response: "final summary"})
		case strings.Contains(req.Prompt, "You are reviewing"):
			json.NewEncoder(w).Encode(generateResponse{Response: "final recommendations"})
		default:
			i := atomic.AddInt32(&actCalls, 1)
			if i == 1 {
				json.NewEncoder(w).Encode(generateResponse{Response: `{"step":"call_tool","tool":"probe","args":{"id":"orders-db"},"reason":"check it"}`})
			} else {
				json.NewEncoder(w).Encode(generateResponse{Response: `{"step":"conclude","conclusion":"orders-db looks fine"}`})
			}
		}
	}))
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "is orders-db healthy?")

	if inv.StoppedReason != "concluded" {
		t.Fatalf("expected StoppedReason=concluded, got %q", inv.StoppedReason)
	}
	if inv.Conclusion != "orders-db looks fine" {
		t.Errorf("expected conclusion to be captured, got %q", inv.Conclusion)
	}
	if len(inv.Steps) != 1 || inv.Steps[0].Tool != "probe" {
		t.Fatalf("expected exactly 1 recorded step calling probe, got %+v", inv.Steps)
	}
	if len(inv.Evidence) != 1 {
		t.Fatalf("expected 1 fact gathered, got %d", len(inv.Evidence))
	}
	if inv.Summary != "final summary" {
		t.Errorf("expected AI summary to be set, got %q", inv.Summary)
	}
	if inv.Recommendations != "final recommendations" {
		t.Errorf("expected AI recommendations to be set, got %q", inv.Recommendations)
	}
}

func TestRun_RejectsUnknownTool(t *testing.T) {
	srv := scriptedOllama(t, []string{
		`{"step":"call_tool","tool":"does_not_exist","args":{}}`,
		`{"step":"conclude","conclusion":"gave up"}`,
	})
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "question")

	if len(inv.Steps) != 0 {
		t.Fatalf("expected no steps executed for an unwhitelisted tool, got %+v", inv.Steps)
	}
	if inv.StoppedReason != "concluded" {
		t.Fatalf("expected the loop to still reach conclude, got %q", inv.StoppedReason)
	}
}

func TestRun_RejectsMissingRequiredArgs(t *testing.T) {
	srv := scriptedOllama(t, []string{
		`{"step":"call_tool","tool":"probe","args":{}}`,
		`{"step":"conclude","conclusion":"gave up"}`,
	})
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "question")

	if len(inv.Steps) != 0 {
		t.Fatalf("expected no steps executed when a required arg is missing, got %+v", inv.Steps)
	}
}

func TestRun_StopsAtMaxIterations(t *testing.T) {
	responses := make([]string, 0, MaxIterations+2)
	for i := 0; i < MaxIterations; i++ {
		responses = append(responses, `{"step":"call_tool","tool":"probe","args":{"id":"x"}}`)
	}
	responses = append(responses, "final summary", "final recommendations")
	srv := scriptedOllama(t, responses)
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "question")

	if inv.StoppedReason != "max_iterations" {
		t.Fatalf("expected StoppedReason=max_iterations, got %q", inv.StoppedReason)
	}
	if len(inv.Steps) != MaxIterations {
		t.Fatalf("expected exactly %d steps, got %d", MaxIterations, len(inv.Steps))
	}
}

func TestRun_StopsWhenModelStuck(t *testing.T) {
	srv := scriptedOllama(t, []string{"not json", "still not json", "nope"})
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "question")

	if inv.StoppedReason != "model_stuck" {
		t.Fatalf("expected StoppedReason=model_stuck, got %q", inv.StoppedReason)
	}
	if len(inv.Steps) != 0 {
		t.Errorf("expected no steps executed, got %+v", inv.Steps)
	}
}

func TestRun_AIUnavailable(t *testing.T) {
	client := ai.NewClient("http://127.0.0.1:1", "test-model")
	registry := NewRegistry(echoTool("probe"))

	inv := Run(context.Background(), client, registry, "question")

	if inv.StoppedReason != "ai_unavailable" {
		t.Fatalf("expected StoppedReason=ai_unavailable, got %q", inv.StoppedReason)
	}
	if inv.SummaryUnavailable == "" {
		t.Error("expected SummaryUnavailable to explain why, got empty")
	}
}

func TestRun_ToolErrorIsRecordedNotFatal(t *testing.T) {
	srv := scriptedOllama(t, []string{
		`{"step":"call_tool","tool":"broken","args":{"id":"x"}}`,
		`{"step":"conclude","conclusion":"done"}`,
	})
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	broken := Tool{
		Name:     "broken",
		Required: []string{"id"},
		Run: func(_ context.Context, _ map[string]string) (ToolResult, error) {
			return ToolResult{}, fmt.Errorf("boom")
		},
	}
	registry := NewRegistry(broken)

	inv := Run(context.Background(), client, registry, "question")

	if len(inv.Steps) != 1 || inv.Steps[0].Err != "boom" {
		t.Fatalf("expected 1 step recording the tool error, got %+v", inv.Steps)
	}
	if inv.StoppedReason != "concluded" {
		t.Fatalf("expected the loop to continue and conclude after a tool error, got %q", inv.StoppedReason)
	}
}

func TestRun_HistoryIncludesPriorSteps(t *testing.T) {
	var secondPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req generateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !strings.Contains(req.Prompt, "read-only infrastructure investigation agent") {
			// a Summarize/Recommend synthesis call, not an Act turn — ignore.
			json.NewEncoder(w).Encode(generateResponse{Response: "synthesis"})
			return
		}
		if strings.Contains(req.Prompt, "No steps have been taken yet") {
			json.NewEncoder(w).Encode(generateResponse{Response: `{"step":"call_tool","tool":"probe","args":{"id":"x"}}`})
			return
		}
		secondPrompt = req.Prompt
		json.NewEncoder(w).Encode(generateResponse{Response: `{"step":"conclude","conclusion":"done"}`})
	}))
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	registry := NewRegistry(echoTool("probe"))

	Run(context.Background(), client, registry, "question")

	if !strings.Contains(secondPrompt, "probe called with id=x") {
		t.Errorf("expected the second turn's prompt to include the first step's result, got: %s", secondPrompt)
	}
}
