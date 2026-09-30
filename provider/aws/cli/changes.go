package cli

import "cloudctl/provider/aws/cli/services"

type ChangesCmd struct {
	AWS services.ChangesAWSCmd `name:"aws" cmd:"" help:"Show a recent CloudTrail change timeline, optionally filtered to one resource"`
}
