package viewer

import (
	"context"
	"errors"
	"fmt"
	"sync"
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

func TestWithNestedProgress_UpdatesExistingSpinnerInsteadOfCreatingNew(t *testing.T) {
	state := &spinnerState{}
	ctx := context.WithValue(context.Background(), spinnerCtxKey{}, state)

	got, err := WithNestedProgress(ctx, "narrating...", func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
	state.mu.Lock()
	gotMessage := state.message
	state.mu.Unlock()
	if gotMessage != "narrating..." {
		t.Errorf("expected the existing spinner state's message to be updated to the nested message, got %q", gotMessage)
	}
}

// TestSetProgress_ConcurrentCallsAreRaceFree guards the fix for a real data
// race: SetProgress used to write spinner.Spinner.Suffix directly, which the
// library's own animation goroutine reads while holding its unexported
// lock (confirmed against briandowns/spinner v1.23.2's Start(), which calls
// PreUpdate and then reads Suffix inside one s.mu-locked section) — so any
// caller goroutine writing Suffix directly, concurrently with a running
// spinner, was an unsynchronized write racing a locked read. SetProgress now
// only ever writes to spinnerState.message under its own mutex; nothing
// outside this package touches a real *spinner.Spinner's Suffix field after
// creation. This test proves that mutex actually serializes concurrent
// SetProgress calls; the render-loop half of the fix (the PreUpdate hook
// installed in WithProgressSpinner copying message into Suffix inside the
// library's own locked section) can only be exercised against a live
// terminal and isn't unit-testable in this suite (same limitation noted for
// the other spinner tests here — term.IsTerminal is always false under `go
// test`), so it's verified by construction against the vendored source
// instead.
func TestSetProgress_ConcurrentCallsAreRaceFree(t *testing.T) {
	state := &spinnerState{}
	ctx := context.WithValue(context.Background(), spinnerCtxKey{}, state)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			SetProgress(ctx, fmt.Sprintf("message %d", i))
		}(i)
	}
	wg.Wait()
}

func TestWithNestedProgress_FallsBackToOwnSpinnerWhenNoneInContext(t *testing.T) {
	got, err := WithNestedProgress(context.Background(), "narrating...", func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}

func TestWithNestedProgress_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := WithNestedProgress(context.Background(), "narrating...", func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the fn's error to propagate, got %v", err)
	}
}
