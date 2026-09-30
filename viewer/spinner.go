package viewer

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"golang.org/x/term"
)

// WithSpinner runs fn while showing an animated spinner with the given
// message, for operations slow enough that users need feedback they're not
// stuck — the local Ollama AI narration call can routinely take anywhere
// from 10 seconds to the full OLLAMA_TIMEOUT default of 2 minutes, and
// previously printed nothing at all while waiting.
//
// The spinner only renders when stdout is an interactive terminal, mirroring
// the same isInteractive discipline already used for credential prompts
// (provider/aws/awsv2.go) — piped/redirected output and test runs get no
// spinner output at all, so this never pollutes non-interactive output or
// test logs.
func WithSpinner[T any](message string, fn func() (T, error)) (T, error) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Suffix = " " + message
		s.Start()
		defer s.Stop()
	}
	return fn()
}

// spinnerCtxKey is the unexported context key WithProgressSpinner stashes
// the running spinner's state under, so SetProgress can find it without any
// exported type leaking into caller signatures.
type spinnerCtxKey struct{}

// spinnerState bundles a running spinner with a mutex-protected pending
// message. SetProgress must never write a *spinner.Spinner's Suffix field
// directly: the library's own animation goroutine (started by Start())
// reads Suffix while holding its unexported lock, so a caller goroutine
// writing Suffix concurrently is an unsynchronized write racing a locked
// read (confirmed against briandowns/spinner v1.23.2's Start(), which locks
// s.mu, invokes PreUpdate, then reads Suffix, all in one critical section).
// Routing every update through message here, copied into Suffix only by the
// PreUpdate hook installed in WithProgressSpinner — which the library calls
// from inside that same locked section — means Suffix is only ever touched
// by the single animation goroutine.
type spinnerState struct {
	mu      sync.Mutex
	message string
}

// WithProgressSpinner is WithSpinner for operations that have distinguishable
// stages (e.g. several sequential AWS calls) rather than one opaque call. It
// runs fn under an animated spinner exactly like WithSpinner, but also
// stashes the running spinner's state into the context passed to fn, so fn —
// or anything it calls — can update the displayed message via SetProgress as
// it moves through stages, instead of leaving one frozen label up for the
// whole duration (which is what reads as "stuck" for a multi-second,
// multi-call fetch).
func WithProgressSpinner[T any](ctx context.Context, message string, fn func(ctx context.Context) (T, error)) (T, error) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		state := &spinnerState{message: message}
		s.Suffix = " " + message
		s.PreUpdate = func(sp *spinner.Spinner) {
			state.mu.Lock()
			defer state.mu.Unlock()
			sp.Suffix = " " + state.message
		}
		s.Start()
		defer s.Stop()
		ctx = context.WithValue(ctx, spinnerCtxKey{}, state)
	}
	return fn(ctx)
}

// SetProgress updates the message of the spinner started by an enclosing
// WithProgressSpinner call, found via ctx. It's a no-op if ctx carries no
// spinner — piped/non-terminal output, tests, or a ctx that was never
// wrapped with WithProgressSpinner — so callers can call it unconditionally
// without checking terminal-ness or nil-guarding themselves.
func SetProgress(ctx context.Context, message string) {
	if state, ok := ctx.Value(spinnerCtxKey{}).(*spinnerState); ok {
		state.mu.Lock()
		state.message = message
		state.mu.Unlock()
	}
}

// WithNestedProgress runs fn under progress reporting appropriate to
// whether ctx already carries a running spinner from an enclosing
// WithProgressSpinner call. If so, it just updates that spinner's message
// (via SetProgress) and calls fn directly — avoiding a second,
// visually-overlapping spinner. If ctx carries no spinner (e.g. a caller
// that invokes a Fetcher directly, bypassing CommandExecutor.Execute —
// see ec2.InvestigateInstanceDef, which the investigation agent's tools
// use), it falls back to WithSpinner's own dedicated spinner so the
// operation still shows a progress indicator on its own.
func WithNestedProgress[T any](ctx context.Context, message string, fn func() (T, error)) (T, error) {
	if _, ok := ctx.Value(spinnerCtxKey{}).(*spinnerState); ok {
		SetProgress(ctx, message)
		return fn()
	}
	return WithSpinner(message, fn)
}
