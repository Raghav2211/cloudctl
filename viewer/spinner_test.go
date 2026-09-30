package viewer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/briandowns/spinner"
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

// withForcedSpinnerFactory forces the interactive-terminal check to true and
// substitutes newSpinner with one that records whether it was called, then
// restores both after the test — the seam TestWithNestedProgress_* tests use
// to actually observe which branch fired, instead of relying on term.IsTerminal
// (always false under `go test`, which is why these tests can't just check
// real stdout output). The returned spinner writes to io.Discard so a forced
// "interactive" run never leaks animation frames into test output even if
// the library's own internal terminal check somehow passed.
func withForcedSpinnerFactory(t *testing.T) *bool {
	t.Helper()
	origInteractive, origFactory := isInteractiveStdout, newSpinner
	created := false
	isInteractiveStdout = func() bool { return true }
	newSpinner = func() *spinner.Spinner {
		created = true
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Writer = io.Discard
		return s
	}
	t.Cleanup(func() { isInteractiveStdout, newSpinner = origInteractive, origFactory })
	return &created
}

func TestWithNestedProgress_UpdatesExistingSpinnerInsteadOfCreatingNew(t *testing.T) {
	created := withForcedSpinnerFactory(t)
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
	if *created {
		t.Error("expected WithNestedProgress to update the existing spinner state instead of creating a new spinner")
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
	created := withForcedSpinnerFactory(t)

	got, err := WithNestedProgress(context.Background(), "narrating...", func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
	if !*created {
		t.Error("expected WithNestedProgress's fallback branch to create its own spinner when ctx carries no spinnerState")
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
