package changes

import (
	"cloudctl/executor"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
)

func NewChangeListCommandExecutor(cfg aws.Config, resourceName, eventName string, excludeEvents []string, since time.Duration, limit int32) *executor.CommandExecutor[*changeListOutput] {
	excludeSet := make(map[string]struct{}, len(excludeEvents))
	for _, name := range excludeEvents {
		excludeSet[name] = struct{}{}
	}
	return &executor.CommandExecutor[*changeListOutput]{
		Fetcher: &changeListFetcher{
			client:        cloudtrail.NewFromConfig(cfg),
			resourceName:  resourceName,
			eventName:     eventName,
			excludeEvents: excludeSet,
			since:         since,
			limit:         limit,
		},
		Viewer: changeListViewer,
	}
}
