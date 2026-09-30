package cli

import (
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/viewer"
	"fmt"
)

// ContextCmd manages named, saved region/profile pairs — a thin CLI layer
// over globals.ListContexts/UseContext/SaveContext/RemoveContext. No AWS
// session is built here; this is pure local state, unlike every other
// command under this package.
type ContextCmd struct {
	Ls   contextLsCmd   `cmd:"" help:"List saved contexts"`
	Use  contextUseCmd  `cmd:"" help:"Switch the active context"`
	Save contextSaveCmd `cmd:"" help:"Save (or update) a named region/profile context"`
	Rm   contextRmCmd   `cmd:"" help:"Remove a saved context"`
}

type contextLsCmd struct{}

func (cmd *contextLsCmd) Run() error {
	contexts := globals.ListContexts()
	if len(contexts) == 0 {
		fmt.Println("No saved contexts yet. Use `ctl context save <name> --region ... --profile ...` to create one.")
		return nil
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle("Saved Contexts")
	tv.AddHeader(viewer.Row{"", "Name", "Region", "Profile"})
	for _, c := range contexts {
		marker := ""
		if c.Active {
			marker = "*"
		}
		tv.AddRow(viewer.Row{marker, c.Name, c.Region, c.Profile})
	}
	tv.View()
	return nil
}

type contextUseCmd struct {
	Name string `arg:"" help:"Context name to switch to"`
}

func (cmd *contextUseCmd) Run() error {
	if err := globals.UseContext(cmd.Name); err != nil {
		return err
	}
	fmt.Printf("Switched to context %q\n", cmd.Name)
	return nil
}

type contextSaveCmd struct {
	Name    string `arg:"" help:"Context name to save"`
	Region  string `name:"region" short:"r" help:"AWS region for this context"`
	Profile string `name:"profile" short:"p" help:"AWS profile for this context"`
}

func (cmd *contextSaveCmd) Run() error {
	globals.SaveContext(cmd.Name, cmd.Region, cmd.Profile)
	fmt.Printf("Saved context %q (region=%q, profile=%q). Run `ctl context use %s` to activate it.\n", cmd.Name, cmd.Region, cmd.Profile, cmd.Name)
	return nil
}

type contextRmCmd struct {
	Name string `arg:"" help:"Context name to remove"`
}

func (cmd *contextRmCmd) Run() error {
	if err := globals.RemoveContext(cmd.Name); err != nil {
		return err
	}
	fmt.Printf("Removed context %q\n", cmd.Name)
	return nil
}
