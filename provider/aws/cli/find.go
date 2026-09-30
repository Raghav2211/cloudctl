package cli

import (
	"cloudctl/provider/aws/cli/services"
	"context"
)

// FindCmd searches the local snapshot (populated by `ctl discover aws`) for
// every resource matching a normalized concept (compute, storage, database,
// ...) across every discovered AWS service — so a user can find "everything
// that's a database" without knowing whether that means RDS or DynamoDB.
type FindCmd struct {
	Concept string `arg:"" enum:"${concepts}" help:"Resource concept to search for"`
}

func (cmd *FindCmd) Run(ctx context.Context) error {
	return services.RunFind(ctx, cmd.Concept)
}
