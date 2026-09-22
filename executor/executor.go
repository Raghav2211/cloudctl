package executor

import (
	"cloudctl/viewer"
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
)

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
	data, err := exe.Fetcher.Fetch(ctx)
	view := exe.Viewer(data, err)
	view.View()
	if view.IsErrorView() {
		return nil
	}
	black := color.New(color.FgGreen)
	boldBlack := black.Add(color.Bold)
	defer func() {
		boldBlack.Println("Time elapsed:", fmt.Sprintf("%.2f", time.Since(start).Seconds()), "sec")
	}()

	return nil
}
