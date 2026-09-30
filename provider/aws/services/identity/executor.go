package identity

import (
	"cloudctl/executor"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func NewWhoamiCommandExecutor(cfg aws.Config, credentialSource string) *executor.CommandExecutor[*whoami] {
	return &executor.CommandExecutor[*whoami]{
		Fetcher: &whoamiFetcher{
			client:           sts.NewFromConfig(cfg),
			region:           cfg.Region,
			credentialSource: credentialSource,
		},
		Viewer: whoamiViewer,
	}
}
