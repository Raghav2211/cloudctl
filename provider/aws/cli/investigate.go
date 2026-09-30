package cli

import "cloudctl/provider/aws/cli/services"

type InvestigateCmd struct {
	AWS services.InvestigateCmd `name:"aws" cmd:"" help:"Run a bounded, read-only investigation agent over a natural-language question"`
}
