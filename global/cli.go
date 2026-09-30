package global

import "cloudctl/executor"

type CLIFlag struct {
	Debug             bool                  `name:"debug" short:"d" help:"Allow debug" negatable:"" default:"0"`
	TZShortIdentifier string                `name:"tz" help:"Configured Timezone in cli output, supported input [utc,los_angeles,tokyo]" default:"utc" required:""`
	Output            executor.OutputFormat `name:"output" short:"o" enum:"table,json,yaml" default:"table" help:"Output format: table, json, or yaml"`
}
