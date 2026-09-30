package cli

import "cloudctl/provider/aws/cli/services"

type WhoamiCmd struct {
	AWS services.WhoamiAWSCmd `name:"aws" cmd:"" help:"Show the currently active AWS identity, account, region, and credential source"`
}
