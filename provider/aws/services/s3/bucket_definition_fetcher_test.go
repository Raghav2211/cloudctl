package s3

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeBucketConfigurationClient implements bucketConfigurationAPI and always
// succeeds, returning minimal-but-valid data for each of the 5 calls.
type fakeBucketConfigurationClient struct{}

func (fakeBucketConfigurationClient) GetBucketPolicy(_ context.Context, _ *s3.GetBucketPolicyInput, _ ...func(*s3.Options)) (*s3.GetBucketPolicyOutput, error) {
	return &s3.GetBucketPolicyOutput{Policy: aws.String(`{"Statement":[]}`)}, nil
}

func (fakeBucketConfigurationClient) GetBucketVersioning(_ context.Context, _ *s3.GetBucketVersioningInput, _ ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	return &s3.GetBucketVersioningOutput{Status: types.BucketVersioningStatusEnabled}, nil
}

func (fakeBucketConfigurationClient) GetBucketTagging(_ context.Context, _ *s3.GetBucketTaggingInput, _ ...func(*s3.Options)) (*s3.GetBucketTaggingOutput, error) {
	return &s3.GetBucketTaggingOutput{TagSet: []types.Tag{{Key: aws.String("team"), Value: aws.String("payments")}}}, nil
}

func (fakeBucketConfigurationClient) GetBucketEncryption(_ context.Context, _ *s3.GetBucketEncryptionInput, _ ...func(*s3.Options)) (*s3.GetBucketEncryptionOutput, error) {
	return &s3.GetBucketEncryptionOutput{ServerSideEncryptionConfiguration: &types.ServerSideEncryptionConfiguration{
		Rules: []types.ServerSideEncryptionRule{{ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{SSEAlgorithm: types.ServerSideEncryptionAes256}}},
	}}, nil
}

func (fakeBucketConfigurationClient) GetBucketLifecycleConfiguration(_ context.Context, _ *s3.GetBucketLifecycleConfigurationInput, _ ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error) {
	return &s3.GetBucketLifecycleConfigurationOutput{Rules: []types.LifecycleRule{{Status: types.ExpirationStatusEnabled}}}, nil
}

// TestBucketConfigurationFetcher_Fetch_NoNilFieldsOnSuccess is a regression
// test for a real production bug: each dimension used two separate channels
// (data + error), both closed by the producing goroutine regardless of
// which one actually received a value. A receive on a closed, empty channel
// is always immediately ready (returns the zero value), so it raced against
// the channel holding the real result in the collecting select — roughly
// half the time per field, the nil-error branch won and neither the data
// nor the error field on bucketDefinition ever got set, leaving Pretty()
// dereferencing a nil *interface{} and panicking (observed live via `ctl s3
// def` against a real bucket). Run many iterations because the original bug
// only manifested probabilistically per field, not deterministically.
func TestBucketConfigurationFetcher_Fetch_NoNilFieldsOnSuccess(t *testing.T) {
	// This test is about the concurrency bug below, not AI behavior (that's
	// covered in ai/ollama_test.go and s3/ai_summary_test.go) — point at a
	// port nothing listens on so Fetch's internal Summarize call fails fast
	// (connection refused) instead of running 200 real LLM round-trips.
	t.Setenv("OLLAMA_HOST", "http://127.0.0.1:1")

	for i := 0; i < 200; i++ {
		f := bucketConfigurationFetcher{client: fakeBucketConfigurationClient{}, bucketName: "my-bucket"}

		def, err := f.Fetch(context.Background())
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}

		if def.policy == nil && def.policyAPIErr == nil {
			t.Fatalf("iteration %d: policy and policyAPIErr are both nil (the exact bug this test guards against)", i)
		}
		if def.version == nil && def.versionAPIErr == nil {
			t.Fatalf("iteration %d: version and versionAPIErr are both nil", i)
		}
		if def.tags == nil && def.tagsAPIError == nil {
			t.Fatalf("iteration %d: tags and tagsAPIError are both nil", i)
		}
		if def.encryptionConfig == nil && def.encryptionConfigAPIError == nil {
			t.Fatalf("iteration %d: encryptionConfig and encryptionConfigAPIError are both nil", i)
		}
		if def.lifecycle == nil && def.lifeCycleAPIError == nil {
			t.Fatalf("iteration %d: lifecycle and lifeCycleAPIError are both nil", i)
		}

		// All 5 calls succeed in this fake, so every field should be the
		// success (data) branch, never the error branch, and Pretty() must
		// render without panicking.
		if def.policyAPIErr != nil || def.versionAPIErr != nil || def.tagsAPIError != nil ||
			def.encryptionConfigAPIError != nil || def.lifeCycleAPIError != nil {
			t.Fatalf("iteration %d: expected no errors on an all-success fetch, got def=%+v", i, def)
		}
		def.Pretty()
	}
}
