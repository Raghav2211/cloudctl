package identity

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// stsAPI is the minimal client capability this package needs, letting tests
// substitute a fake instead of a real *sts.Client (ADR-0007).
type stsAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type whoamiFetcher struct {
	client           stsAPI
	region           string
	credentialSource string
}

// Fetch calls STS GetCallerIdentity — the same API AWS's own credential
// resolution implicitly relies on — to confirm credentials are valid and
// AWS is reachable, and to report exactly which account/identity they
// resolve to before any other command runs against them.
func (f whoamiFetcher) Fetch(ctx context.Context) (*whoami, error) {
	out, err := f.client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	return newWhoami(out.Account, out.Arn, out.UserId, f.region, f.credentialSource), nil
}
