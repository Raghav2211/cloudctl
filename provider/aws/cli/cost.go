package cli

import "cloudctl/provider/aws/cli/services"

type CostCmd struct {
	AWS services.CostCmd `name:"aws" cmd:"" help:"Analyze cost: real spend via Cost Explorer, or a per-resource idle-waste scan as a fallback"`
}
