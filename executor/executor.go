package executor

import (
	"cloudctl/fetcher"
	"cloudctl/viewer"
	"fmt"
	"time"

	"github.com/fatih/color"
)

type CommandExecutor struct {
	Fetcher fetcher.Fetcher
	Viewer  viewer.ViewerFunc
}

func (exe *CommandExecutor) Execute() error {
	start := time.Now()
	data, err := exe.Fetcher.Fetch()
	if err != nil {
		return err
	}
	if exe.Viewer != nil {
		viewType := exe.Viewer(data)
		viewType.View()
	}

	black := color.New(color.FgGreen)
	boldBlack := black.Add(color.Bold)
	defer boldBlack.Println("Time elapsed:", fmt.Sprintf("%.2f", time.Since(start).Seconds()), "sec")

	return nil
}
