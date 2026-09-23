package viewer

import (
	"context"
	"errors"
	"testing"
)

func TestWithProgressSpinner_ReturnsFnResult(t *testing.T) {
	got, err := WithProgressSpinner(context.Background(), "Fetching...", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}

func TestWithProgressSpinner_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := WithProgressSpinner(context.Background(), "Fetching...", func(ctx context.Context) (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the fn's error to propagate, got %v", err)
	}
}

// TestSetProgress_NoSpinnerInContext confirms SetProgress never panics or
// blocks when called on a plain context — the no-op path a fetcher hits
// whenever output isn't a terminal or it wasn't wrapped in
// WithProgressSpinner (e.g. in every test in this codebase).
func TestSetProgress_NoSpinnerInContext(t *testing.T) {
	SetProgress(context.Background(), "should be a no-op")
}
