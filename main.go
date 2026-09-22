package main

import (
	"cloudctl/global"
	"cloudctl/provider/aws/cli"
	"context"
	"os"
	"os/signal"

	"github.com/alecthomas/kong"
)

type CLI struct {
	global.CLIFlag
	AWS      cli.AWSCmd      `name:"aws" cmd:"" help:"AWS cloud provider commands"`
	Discover cli.DiscoverCmd `name:"discover" cmd:"" help:"Discover and snapshot infrastructure"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cli := CLI{
		CLIFlag: global.CLIFlag{},
	}
	kongCtx := kong.Parse(&cli,
		kong.Name("cloudctl"),
		kong.Description("cloudctl is a GO library that interacts with cloud providers and displays output in a human-readable fashion."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.Vars{
			"version": "0.0.1",
		})
	err := kongCtx.Run(&cli.CLIFlag)
	kongCtx.FatalIfErrorf(err)
}
