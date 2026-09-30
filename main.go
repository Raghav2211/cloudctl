package main

import (
	"cloudctl/concept"
	"cloudctl/executor"
	"cloudctl/global"
	"cloudctl/provider/aws/cli"
	"context"
	"os"
	"os/signal"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/willabides/kongplete"
)

type CLI struct {
	global.CLIFlag
	AWS                cli.AWSCmd                   `name:"aws" cmd:"" help:"AWS cloud provider commands"`
	Discover           cli.DiscoverCmd              `name:"discover" cmd:"" help:"Discover and snapshot infrastructure"`
	Whoami             cli.WhoamiCmd                `name:"whoami" cmd:"" help:"Show the currently active cloud identity"`
	Context            cli.ContextCmd               `name:"context" cmd:"" help:"Manage saved region/profile contexts"`
	Find               cli.FindCmd                  `name:"find" cmd:"" help:"Find discovered resources by concept (compute, storage, database, ...)"`
	Changes            cli.ChangesCmd               `name:"changes" cmd:"" help:"Show a recent change timeline (CloudTrail)"`
	Ask                cli.AskCmd                   `name:"ask" cmd:"" help:"Ask a natural-language question, mapped to a supported read-only query"`
	Investigate        cli.InvestigateCmd           `name:"investigate" cmd:"" help:"Run a bounded, read-only investigation agent over a natural-language question"`
	Cost               cli.CostCmd                  `name:"cost" cmd:"" help:"Analyze cost: real spend via Cost Explorer, or a per-resource idle-waste scan as a fallback"`
	InstallCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions (run: eval \"$(cloudctl install-completions)\")"`
}

func conceptNames() string {
	all := concept.All()
	names := make([]string, len(all))
	for i, c := range all {
		names[i] = string(c)
	}
	return strings.Join(names, ",")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cli := CLI{
		CLIFlag: global.CLIFlag{},
	}
	parser := kong.Must(&cli,
		kong.Name("cloudctl"),
		kong.Description("cloudctl is a GO library that interacts with cloud providers and displays output in a human-readable fashion."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.Vars{
			"version":  "0.0.1",
			"concepts": conceptNames(),
		})

	// kongplete.Complete intercepts shell completion requests (COMP_LINE
	// set) and exits before reaching normal parsing/execution; it's a no-op
	// for every regular invocation.
	kongplete.Complete(parser)

	kongCtx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	executor.SetOutputFormat(cli.CLIFlag.Output)
	err = kongCtx.Run(&cli.CLIFlag)
	kongCtx.FatalIfErrorf(err)
}
