// Package ai talks to a local Ollama server to turn deterministic evidence
// into prose. Per ADR-009, this is deliberately not a generic LLMProvider
// abstraction — Client talks to exactly one backend. Introduce an
// abstraction only when a second concrete backend is an actual need.
package ai

import (
	"bytes"
	"cloudctl/evidence"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultModel   = "deepseek-r1:7b"
	// defaultTimeout is generous because real model inference — especially a
	// "thinking" model producing chain-of-thought before its answer, or a
	// cold start that has to load the model into memory first — routinely
	// takes well past a naive 30s HTTP timeout even though the Ollama
	// server itself is already up and responsive.
	defaultTimeout = 120 * time.Second
)

// Client summarizes evidence via a local Ollama server's /api/generate
// endpoint.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewClientFromEnv builds a Client from OLLAMA_HOST (default
// "http://localhost:11434"), OLLAMA_MODEL (default "deepseek-r1:7b" —
// override with whatever model is actually pulled locally), and
// OLLAMA_TIMEOUT (a duration string like "3m", default 2 minutes — override
// if your hardware/model needs longer than that per call).
func NewClientFromEnv() *Client {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = defaultModel
	}
	client := NewClient(baseURL, model)
	if raw := os.Getenv("OLLAMA_TIMEOUT"); raw != "" {
		if timeout, err := time.ParseDuration(raw); err == nil {
			client.http.Timeout = timeout
		}
	}
	return client
}

// NewClient builds a Client against a specific Ollama base URL and model,
// letting tests point at an httptest.Server instead of a real server.
func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: defaultTimeout},
	}
}

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type generateResponse struct {
	Response string `json:"response"`
	Error    string `json:"error"`
}

// Summarize turns already-fetched, already-correct evidence into a concise,
// risk-focused prose summary.
//
// The returned string is not itself an evidence.Evidence value and carries
// no Confidence tag — callers that fold this text back into an Evidence list
// (e.g. to render or store it) must tag it evidence.Hypothesis or
// evidence.Recommendation themselves. Summarize never assigns Confidence on
// the model's behalf.
func (c *Client) Summarize(ctx context.Context, facts []evidence.Evidence) (string, error) {
	if len(facts) == 0 {
		return "", fmt.Errorf("summarize: no evidence provided")
	}
	return c.generate(ctx, "summarize", buildPrompt(facts))
}

// Recommend turns already-fetched, already-correct evidence into concrete,
// evidence-grounded security/reliability/cost recommendations.
//
// Like Summarize, the returned string carries no Confidence tag — callers
// that fold this text back into an Evidence list must tag it
// evidence.Recommendation themselves (evidence.go's rule that only calling
// code may assign that tag, never the model itself).
func (c *Client) Recommend(ctx context.Context, facts []evidence.Evidence) (string, error) {
	if len(facts) == 0 {
		return "", fmt.Errorf("recommend: no evidence provided")
	}
	return c.generate(ctx, "recommend", buildRecommendPrompt(facts))
}

// generate is the shared /api/generate request/response plumbing behind
// both Summarize and Recommend — only the prompt and error-message prefix
// differ between the two.
func (c *Client) generate(ctx context.Context, action, prompt string) (string, error) {
	body, err := json.Marshal(generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("%s: encoding request: %w", action, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%s: building request: %w", action, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: calling ollama at %s: %w", action, c.baseURL, err)
	}
	defer resp.Body.Close()

	var out generateResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&out); decodeErr != nil {
		return "", fmt.Errorf("%s: decoding ollama response (status %d): %w", action, resp.StatusCode, decodeErr)
	}
	if resp.StatusCode != http.StatusOK {
		if out.Error != "" {
			return "", fmt.Errorf("%s: ollama error: %s", action, out.Error)
		}
		return "", fmt.Errorf("%s: ollama returned status %d", action, resp.StatusCode)
	}
	if strings.TrimSpace(out.Response) == "" {
		return "", fmt.Errorf("%s: ollama returned an empty response", action)
	}
	return strings.TrimSpace(out.Response), nil
}

func buildPrompt(facts []evidence.Evidence) string {
	var b strings.Builder
	b.WriteString("You are summarizing infrastructure configuration evidence for an engineer.\n")
	b.WriteString("Only state what is directly supported by the evidence listed below. Do not invent facts, resource names, or values that are not listed.\n")
	b.WriteString("Write a concise, risk-focused summary in plain English, a few sentences.\n\n")
	b.WriteString("Evidence:\n")
	for _, f := range facts {
		fmt.Fprintf(&b, "- [%s] %s = %v (source: %s)\n", f.Confidence, f.Field, f.Value, f.Source)
	}
	return b.String()
}

func buildRecommendPrompt(facts []evidence.Evidence) string {
	var b strings.Builder
	b.WriteString("You are reviewing infrastructure configuration evidence for an engineer.\n")
	b.WriteString("Only reference resources, fields, and values directly supported by the evidence listed below. Do not invent facts, resource names, or values that are not listed.\n")
	b.WriteString("Identify concrete, actionable security, reliability, or cost improvements suggested by this evidence, as short imperative sentences. If nothing stands out, say so plainly instead of inventing a recommendation.\n\n")
	b.WriteString("Evidence:\n")
	for _, f := range facts {
		fmt.Fprintf(&b, "- [%s] %s = %v (source: %s)\n", f.Confidence, f.Field, f.Value, f.Source)
	}
	return b.String()
}
