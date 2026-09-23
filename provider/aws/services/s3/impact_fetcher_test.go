package s3

import (
	"cloudctl/snapshot"
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// isolateSnapshotStore points CLOUDCTL_SNAPSHOT_DB at a fresh temp file so
// tests never read or write the real user's ~/.cloudctl/snapshot.db.
func isolateSnapshotStore(t *testing.T) {
	t.Helper()
	t.Setenv("CLOUDCTL_SNAPSHOT_DB", filepath.Join(t.TempDir(), "snapshot.db"))
}

// fakeIAMClient implements iamPolicyCrossReferenceAPI with canned,
// per-policy-ARN responses, letting tests exercise the full Fetch chain
// (paginate, decode, match, cross-reference) without a real account.
type fakeIAMClient struct {
	policies    []types.Policy
	versions    map[string]string // policyArn -> URL-encoded document
	entities    map[string]iam.ListEntitiesForPolicyOutput
	listErr     error
	versionErrs map[string]error
}

func (f fakeIAMClient) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return &iam.ListPoliciesOutput{Policies: f.policies}, nil
}

func (f fakeIAMClient) GetPolicyVersion(_ context.Context, params *iam.GetPolicyVersionInput, _ ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error) {
	arn := *params.PolicyArn
	if err, ok := f.versionErrs[arn]; ok {
		return nil, err
	}
	doc, ok := f.versions[arn]
	if !ok {
		return &iam.GetPolicyVersionOutput{}, nil
	}
	return &iam.GetPolicyVersionOutput{PolicyVersion: &types.PolicyVersion{Document: aws.String(doc)}}, nil
}

func (f fakeIAMClient) ListEntitiesForPolicy(_ context.Context, params *iam.ListEntitiesForPolicyInput, _ ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error) {
	out := f.entities[*params.PolicyArn]
	return &out, nil
}

func encodedDoc(t *testing.T, raw string) string {
	t.Helper()
	return url.QueryEscape(raw)
}

// TestBucketImpactFetcher_Fetch_DedupesRepeatedActionsAcrossStatements is a
// regression test for a real bug found via live testing: Terraform-generated
// policies routinely have one statement per resource/principal pair, so a
// policy with many matching statements repeated the same action dozens of
// times before dedupeStrings was introduced — one real policy in the test
// account rendered a single ~17,000-character table row as a result.
func TestBucketImpactFetcher_Fetch_DedupesRepeatedActionsAcrossStatements(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	isolateSnapshotStore(t)

	policyArn := "arn:aws:iam::123456789012:policy/many-statements"
	doc := `{"Statement":[
		{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::my-bucket/*"},
		{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::my-bucket/*"},
		{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::my-bucket/*"},
		{"Effect":"Allow","Action":"s3:ListBucket","Resource":"arn:aws:s3:::my-bucket"}
	]}`

	client := fakeIAMClient{
		policies: []types.Policy{
			{Arn: aws.String(policyArn), PolicyName: aws.String("many-statements"), DefaultVersionId: aws.String("v1")},
		},
		versions: map[string]string{policyArn: encodedDoc(t, doc)},
	}

	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}
	impact, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(impact.matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(impact.matches))
	}
	actions := impact.matches[0].actions
	if len(actions) != 2 {
		t.Fatalf("expected actions deduped to [s3:GetObject, s3:ListBucket], got %v", actions)
	}
}

func TestBucketImpactFetcher_Fetch_MatchesPolicyAndCrossReferencesPrincipals(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	isolateSnapshotStore(t)

	// Seed a snapshot so saveRelationships has somewhere to write —
	// deterministic, not dependent on whatever the real store happens to
	// contain.
	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		t.Fatalf("failed to open isolated snapshot store: %v", err)
	}
	if _, err := store.NewSnapshot(context.Background(), "aws"); err != nil {
		t.Fatalf("failed to seed a snapshot: %v", err)
	}
	store.Close()

	policyArn := "arn:aws:iam::123456789012:policy/my-bucket-read"
	doc := `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:ListBucket"],"Resource":["arn:aws:s3:::my-bucket","arn:aws:s3:::my-bucket/*"]}]}`

	client := fakeIAMClient{
		policies: []types.Policy{
			{Arn: aws.String(policyArn), PolicyName: aws.String("my-bucket-read"), DefaultVersionId: aws.String("v1")},
		},
		versions: map[string]string{policyArn: encodedDoc(t, doc)},
		entities: map[string]iam.ListEntitiesForPolicyOutput{
			policyArn: {PolicyRoles: []types.PolicyRole{{RoleName: aws.String("payments-service")}}},
		},
	}

	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}
	impact, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(impact.matches) != 1 {
		t.Fatalf("expected 1 match, got %d: %+v", len(impact.matches), impact.matches)
	}
	m := impact.matches[0]
	if m.policyName != "my-bucket-read" {
		t.Errorf("expected policyName 'my-bucket-read', got %q", m.policyName)
	}
	if m.effect != "Allow" {
		t.Errorf("expected effect 'Allow', got %q", m.effect)
	}
	if len(m.principals) != 1 || m.principals[0] != "role/payments-service" {
		t.Errorf("expected principals [role/payments-service], got %v", m.principals)
	}
	if impact.relationshipsError != "" {
		t.Errorf("expected relationships to persist cleanly, got error: %s", impact.relationshipsError)
	}
	if impact.relationshipsSaved != 1 {
		t.Errorf("expected 1 relationship saved (1 match x 1 principal), got %d", impact.relationshipsSaved)
	}
	if impact.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to be set (Ollama unreachable)")
	}

	// Must render without panicking.
	bucketImpactViewer(impact, nil).View()
}

func TestBucketImpactFetcher_Fetch_NoMatchingPolicies(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	isolateSnapshotStore(t)

	policyArn := "arn:aws:iam::123456789012:policy/unrelated"
	doc := `{"Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::some-other-bucket/*"}]}`

	client := fakeIAMClient{
		policies: []types.Policy{
			{Arn: aws.String(policyArn), PolicyName: aws.String("unrelated"), DefaultVersionId: aws.String("v1")},
		},
		versions: map[string]string{policyArn: encodedDoc(t, doc)},
	}

	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}
	impact, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(impact.matches) != 0 {
		t.Fatalf("expected 0 matches, got %d: %+v", len(impact.matches), impact.matches)
	}
	if impact.aiSummaryUnavailable == "" {
		t.Error("expected aiSummaryUnavailable to be set for a bucket with no evidence")
	}
	if impact.aiSummary != "" {
		t.Error("expected no AI summary to be attempted with zero evidence")
	}

	bucketImpactViewer(impact, nil).View()
}

func TestBucketImpactFetcher_Fetch_DegradesGracefullyOnPerPolicyErrors(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	isolateSnapshotStore(t)

	badArn := "arn:aws:iam::123456789012:policy/broken"
	goodArn := "arn:aws:iam::123456789012:policy/good"
	doc := `{"Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::my-bucket/*"}]}`

	client := fakeIAMClient{
		policies: []types.Policy{
			{Arn: aws.String(badArn), PolicyName: aws.String("broken"), DefaultVersionId: aws.String("v1")},
			{Arn: aws.String(goodArn), PolicyName: aws.String("good"), DefaultVersionId: aws.String("v1")},
		},
		versions:    map[string]string{goodArn: encodedDoc(t, doc)},
		versionErrs: map[string]error{badArn: context.DeadlineExceeded},
	}

	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}
	impact, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("expected a broken individual policy not to fail the whole fetch: %v", err)
	}
	if len(impact.matches) != 1 || impact.matches[0].policyName != "good" {
		t.Fatalf("expected exactly the 'good' policy to match, got %+v", impact.matches)
	}
}

// TestBucketImpactFetcher_Fetch_NoSnapshotYet_GracefulDegradation confirms
// that a match found before anyone has ever run `ctl discover aws` (so no
// snapshot exists to write relationships into) still renders cleanly — the
// command's primary purpose (surfacing the IAM cross-reference) doesn't
// depend on persistence succeeding.
func TestBucketImpactFetcher_Fetch_NoSnapshotYet_GracefulDegradation(t *testing.T) {
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")
	isolateSnapshotStore(t) // fresh store, migrated schema, but zero snapshots

	policyArn := "arn:aws:iam::123456789012:policy/my-bucket-read"
	doc := `{"Statement":[{"Effect":"Allow","Action":"s3:GetObject","Resource":"arn:aws:s3:::my-bucket/*"}]}`
	client := fakeIAMClient{
		policies: []types.Policy{
			{Arn: aws.String(policyArn), PolicyName: aws.String("my-bucket-read"), DefaultVersionId: aws.String("v1")},
		},
		versions: map[string]string{policyArn: encodedDoc(t, doc)},
	}

	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}
	impact, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(impact.matches) != 1 {
		t.Fatalf("expected the match to still be found, got %d matches", len(impact.matches))
	}
	if impact.relationshipsError == "" {
		t.Error("expected relationshipsError to explain that no snapshot exists yet")
	}
	if impact.relationshipsSaved != 0 {
		t.Errorf("expected 0 relationships saved with no snapshot to write into, got %d", impact.relationshipsSaved)
	}

	bucketImpactViewer(impact, nil).View() // must not panic
}

func TestBucketImpactFetcher_Fetch_ListPoliciesAPIError(t *testing.T) {
	client := fakeIAMClient{listErr: context.DeadlineExceeded}
	f := bucketImpactFetcher{client: client, bucketName: "my-bucket"}

	_, err := f.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected an error from a failing ListPolicies call, got nil")
	}
}

func TestStatementActionsForBucket(t *testing.T) {
	target := bucketARN("my-bucket")
	cases := []struct {
		name      string
		resources []string
		actions   []string
		want      []string
	}{
		{"exact bucket ARN, any action passes through unfiltered", []string{target}, []string{"eks:DescribeCluster"}, []string{"eks:DescribeCluster"}},
		{"bucket object ARN, any action passes through unfiltered", []string{target + "/*"}, []string{"logs:PutLogEvents"}, []string{"logs:PutLogEvents"}},
		{"wildcard resource, s3 action", []string{"*"}, []string{"s3:GetObject"}, []string{"s3:GetObject"}},
		{"wildcard resource, s3 wildcard action", []string{"*"}, []string{"s3:*"}, []string{"s3:*"}},
		{"wildcard resource, fully-wildcard action", []string{"*"}, []string{"*"}, []string{"*"}},
		{"wildcard resource, unrelated service action — false positive this fix prevents", []string{"*"}, []string{"eks:DescribeCluster"}, nil},
		{
			"wildcard resource, broad admin policy — only the s3-relevant action survives (the sre-ops-sso case found live)",
			[]string{"*"}, []string{"workspaces:*", "wafv2:*", "s3:*", "sts:*"}, []string{"s3:*"},
		},
		{"different bucket entirely", []string{"arn:aws:s3:::my-bucket-other"}, []string{"s3:GetObject"}, nil},
		{"different bucket, object ARN", []string{"arn:aws:s3:::my-bucket-other/*"}, []string{"s3:GetObject"}, nil},
		{"bucket name as a path segment doesn't count", []string{"arn:aws:s3:::other/my-bucket/*"}, []string{"s3:GetObject"}, nil},
		{"mixed resources, one relevant", []string{"arn:aws:s3:::unrelated", target}, []string{"s3:GetObject"}, []string{"s3:GetObject"}},
	}
	for _, c := range cases {
		got := statementActionsForBucket(c.resources, c.actions, target)
		if len(got) != len(c.want) {
			t.Errorf("%s: statementActionsForBucket(%v, %v) = %v, want %v", c.name, c.resources, c.actions, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: statementActionsForBucket(%v, %v) = %v, want %v", c.name, c.resources, c.actions, got, c.want)
				break
			}
		}
	}
}
