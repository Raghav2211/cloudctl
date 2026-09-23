package executor

import (
	"cloudctl/viewer"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fatih/color"
)

// ErrCommandFailed is returned by Execute when the rendered view represents
// a real failure (see Viewer.IsFailure) but the originating Fetcher error is
// nil — e.g. a multi-section view where only one sub-panel failed. Where a
// concrete Fetcher error exists, Execute returns that instead so callers get
// the real, inspectable error rather than this generic sentinel.
var ErrCommandFailed = errors.New("command failed")

// Fetcher retrieves the data a command needs to render. Implementations
// should treat ctx as cancellable (e.g. via SDK calls that accept it) rather
// than starting their own background context.
type Fetcher[T any] interface {
	Fetch(ctx context.Context) (T, error)
}

type CommandExecutor[T any] struct {
	Fetcher Fetcher[T]
	Viewer  viewer.ViewerFunc[T]
}

func (exe *CommandExecutor[T]) Execute(ctx context.Context) error {
	start := time.Now()
	data, fetchErr := exe.Fetcher.Fetch(ctx)
	view := exe.Viewer(data, fetchErr)
	view.View()

	// cmdErr drives the process exit code (via the caller's Run() -> kong ->
	// FatalIfErrorf). It's computed independently of IsErrorView(), which
	// only controls whether the "Time elapsed" footer prints below and keeps
	// its original behavior (true for every severity) unchanged.
	var cmdErr error
	if view.IsFailure() {
		if fetchErr != nil {
			cmdErr = fetchErr
		} else {
			cmdErr = ErrCommandFailed
		}
	}

	if view.IsErrorView() {
		return cmdErr
	}
	black := color.New(color.FgGreen)
	boldBlack := black.Add(color.Bold)
	defer func() {
		boldBlack.Println("Time elapsed:", fmt.Sprintf("%.2f", time.Since(start).Seconds()), "sec")
	}()

	return cmdErr
}
