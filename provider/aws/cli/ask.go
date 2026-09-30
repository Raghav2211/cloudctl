package cli

import "cloudctl/provider/aws/cli/services"

type AskCmd struct {
	AWS services.AskCmd `name:"aws" cmd:"" help:"Ask a natural-language question, mapped to a supported read-only query"`
}
