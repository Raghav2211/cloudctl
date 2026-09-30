package changes

import (
	"cloudctl/evidence"
	"cloudctl/viewer"
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// InvestigateChanges runs the same fetch as `ctl changes aws`, for use by
// the investigation agent (`ctl investigate`).
func InvestigateChanges(ctx context.Context, cfg aws.Config, resourceName, eventName string, since time.Duration, limit int32) ([]evidence.Evidence, string, error) {
	exec := NewChangeListCommandExecutor(cfg, resourceName, eventName, nil, since, limit)
	data, err := exec.Fetcher.Fetch(ctx)
	if err != nil {
		return nil, "", err
	}
	return changeListEvidence(data), viewer.StructuredJSON(exec.Viewer(data, nil)), nil
}
