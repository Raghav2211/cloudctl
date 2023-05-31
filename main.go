package main

import (
	"cloudctl/provider/aws/cli"
	"cloudctl/provider/aws/cli/globals"

	"github.com/alecthomas/kong"
)

type CLI struct {
	globals.CLIFlag
	AWS cli.AWSCmd `name:"aws" cmd:"" help:"AWS cloud provider commands"`
}

func main() {

	cli := CLI{
		CLIFlag: globals.CLIFlag{},
	}
	ctx := kong.Parse(&cli,
		kong.Name("cloudctl"),
		kong.Description("cloudctl is a GO library that interacts with cloud providers and displays output in a human-readable fashion."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Compact: true,
		}),
		kong.Vars{
			"version": "0.0.1",
		})
	err := ctx.Run(&cli.CLIFlag)
	ctx.FatalIfErrorf(err)

}
