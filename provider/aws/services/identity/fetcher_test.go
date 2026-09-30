package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// fakeSTSClient implements stsAPI with a canned response, letting tests
// substitute it for a real *sts.Client (ADR-0007).
type fakeSTSClient struct {
	out *sts.GetCallerIdentityOutput
	err error
}

func (f *fakeSTSClient) GetCallerIdentity(_ context.Context, _ *sts.GetCallerIdentityInput, _ ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func TestWhoamiFetcher_Fetch_HappyPath(t *testing.T) {
	account := "123456789012"
	arn := "arn:aws:iam::123456789012:user/alice"
	userID := "AIDAEXAMPLE"
	client := &fakeSTSClient{out: &sts.GetCallerIdentityOutput{Account: &account, Arn: &arn, UserId: &userID}}
	f := whoamiFetcher{client: client, region: "eu-west-1", credentialSource: "SharedConfigCredentials"}

	got, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if derefStr(got.account) != account {
		t.Errorf("expected account %q, got %q", account, derefStr(got.account))
	}
	if derefStr(got.arn) != arn {
		t.Errorf("expected arn %q, got %q", arn, derefStr(got.arn))
	}
	if got.region != "eu-west-1" {
		t.Errorf("expected region %q, got %q", "eu-west-1", got.region)
	}
	if got.credentialSource != "SharedConfigCredentials" {
		t.Errorf("expected credential source %q, got %q", "SharedConfigCredentials", got.credentialSource)
	}

	whoamiViewer(got, nil).View() // must not panic
}

func TestWhoamiFetcher_Fetch_APIError(t *testing.T) {
	client := &fakeSTSClient{err: errors.New("boom")}
	f := whoamiFetcher{client: client}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing GetCallerIdentity call, got nil")
	}

	whoamiViewer(nil, err).View() // must not panic
}
