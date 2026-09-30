package services

import (
	"cloudctl/concept"
	"cloudctl/snapshot"
	"cloudctl/viewer"
	"context"
	"fmt"
)

// RunFind searches the local snapshot (populated by `ctl discover aws`) for
// every resource matching a normalized concept (compute, storage, database,
// ...) across every discovered AWS service — so a caller can find
// "everything that's a database" without knowing whether that means RDS or
// DynamoDB. Shared by `ctl find <concept>` and `ctl ask`, which both need
// this exact same lookup.
func RunFind(ctx context.Context, conceptName string) error {
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		return err
	}
	defer store.Close()

	snapshotID, err := store.LatestSnapshotID(ctx, "aws")
	if err != nil {
		return err
	}

	c := concept.Concept(conceptName)
	resourceTypes := concept.ResourceTypes(c)

	var all []snapshot.StoredResource
	for _, t := range resourceTypes {
		resources, err := store.ListResources(ctx, snapshotID, t)
		if err != nil {
			return err
		}
		all = append(all, resources...)
	}

	if len(all) == 0 {
		fmt.Printf("No %s resources found in the latest snapshot. Run `ctl discover aws` again if the account has since changed.\n", conceptName)
		return nil
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Resources matching concept %q", conceptName))
	tv.AddHeader(viewer.Row{"ID", "Type"})
	for _, r := range all {
		tv.AddRow(viewer.Row{r.ID, r.Type})
	}
	tv.View()
	return nil
}
