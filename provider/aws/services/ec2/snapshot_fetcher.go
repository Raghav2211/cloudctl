package ec2

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/snapshot"
	ctltime "cloudctl/time"
	"cloudctl/viewer"
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// instanceListFromSnapshotFetcher reads instances from the latest local
// snapshot instead of calling AWS live, reusing newInstanceSummary — the
// exact constructor the live path uses — so any JSON round-trip data loss
// would show up immediately as a rendering difference, not require separate
// verification logic (ADR-008).
type instanceListFromSnapshotFetcher struct {
	store  *snapshot.Store
	tz     *ctltime.Timezone
	filter InstanceListFilter
}

func (f instanceListFromSnapshotFetcher) Fetch(ctx context.Context) (*instanceListOutput, error) {
	defer f.store.Close()

	snapshotID, err := f.store.LatestSnapshotID(ctx, "aws")
	if err != nil {
		return nil, ctlaws.NewErrorInfo(err, viewer.ERROR, nil)
	}
	stored, err := f.store.ListResources(ctx, snapshotID, "ec2:instance")
	if err != nil {
		return nil, ctlaws.NewErrorInfo(err, viewer.ERROR, nil)
	}
	if len(stored) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}

	instancesByState := make(map[string][]*instanceSummary)
	for _, r := range stored {
		var inst types.Instance
		if err := json.Unmarshal(r.AttrsJSON, &inst); err != nil {
			return nil, ctlaws.NewErrorInfo(fmt.Errorf("decoding snapshot resource %s: %w", r.ID, err), viewer.ERROR, nil)
		}
		if !f.filter.applyCustomFilter(inst) {
			continue
		}
		state := string(inst.State.Name)
		instancesByState[state] = append(instancesByState[state], newInstanceSummary(inst, f.tz))
	}
	if len(instancesByState) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}
	return &instanceListOutput{instancesByState: instancesByState}, nil
}
