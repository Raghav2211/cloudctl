package globals

type CLIFlag struct {
	Profile           string `name:"profile" short:"p" help:"Set AWS profile" default:""`
	Region            string `name:"region" short:"r" help:"Set AWS Region" default:""`
	Debug             bool   `name:"debug" short:"d" help:"Allow debug" negatable:""`
	TZShortIdentifier string `name:"tz" help:"Configured Timezne in aws output, supported input [utc,los_angeles,tokyo]" default:"utc" required:""`
}
