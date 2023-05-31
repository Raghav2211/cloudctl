package main

import (
	"cloudctl/provider/aws/cli"
	"cloudctl/provider/aws/cli/globals"

	"github.com/alecthomas/kong"
)

type CLI struct {
	globals.CLIFlag
	S3  cli.S3Command  `name:"s3" cmd:"" help:"operation on S3 buckets"`
	EC2 cli.EC2Command `name:"ec2" cmd:"" help:"operation on ec2"`
}

func main() {

	cli := CLI{
		CLIFlag: globals.CLIFlag{},
	}
	ctx := kong.Parse(&cli,
		kong.Name("raws"),
		kong.Description("CLI to read aws resources and provide output in human redable format"),
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
