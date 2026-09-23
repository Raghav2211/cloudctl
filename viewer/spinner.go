package viewer

import (
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
