package viewer

import (
	"context"
	"os"
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
// the running spinner under, so SetProgress can find it without any
// exported type leaking into caller signatures.
type spinnerCtxKey struct{}

// WithProgressSpinner is WithSpinner for operations that have distinguishable
// stages (e.g. several sequential AWS calls) rather than one opaque call. It
// runs fn under an animated spinner exactly like WithSpinner, but also
// stashes the running spinner into the context passed to fn, so fn — or
// anything it calls — can update the displayed message via SetProgress as it
// moves through stages, instead of leaving one frozen label up for the whole
// duration (which is what reads as "stuck" for a multi-second, multi-call
// fetch).
func WithProgressSpinner[T any](ctx context.Context, message string, fn func(ctx context.Context) (T, error)) (T, error) {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
		s.Suffix = " " + message
		s.Start()
		defer s.Stop()
		ctx = context.WithValue(ctx, spinnerCtxKey{}, s)
	}
	return fn(ctx)
}

// SetProgress updates the message of the spinner started by an enclosing
// WithProgressSpinner call, found via ctx. It's a no-op if ctx carries no
// spinner — piped/non-terminal output, tests, or a ctx that was never
// wrapped with WithProgressSpinner — so callers can call it unconditionally
// without checking terminal-ness or nil-guarding themselves.
func SetProgress(ctx context.Context, message string) {
	if s, ok := ctx.Value(spinnerCtxKey{}).(*spinner.Spinner); ok {
		s.Suffix = " " + message
	}
}
