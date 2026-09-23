package s3

import (
	ctltime "cloudctl/time"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// TestNewBucketObjectOutput_StorageClassReflectsActualValue is a regression
// test for a real bug found via architecture review: the old implementation
// read `o.StorageClass.Values()[0]`, but Values() is a static method that
// lists every possible enum constant regardless of the receiver — it always
// evaluated to "STANDARD", silently misreporting every object whose real
// storage class was anything else (GLACIER, INTELLIGENT_TIERING, ...).
func TestNewBucketObjectOutput_StorageClassReflectsActualValue(t *testing.T) {
	cases := []types.ObjectStorageClass{
		types.ObjectStorageClassStandard,
		types.ObjectStorageClassGlacier,
		types.ObjectStorageClassIntelligentTiering,
		types.ObjectStorageClassDeepArchive,
	}

	tz := ctltime.GetTZ("utc")
	for _, sc := range cases {
		lastModified := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		o := types.Object{
			Key:          aws.String("some/key"),
			Size:         aws.Int64(123),
			StorageClass: sc,
			LastModified: &lastModified,
		}
		out := newBucketObjectOutput(o, tz)
		if out.storageClass == nil || *out.storageClass != string(sc) {
			got := "nil"
			if out.storageClass != nil {
				got = *out.storageClass
			}
			t.Errorf("expected storageClass %q, got %q (the exact bug this test guards against: always reporting %q)",
				sc, got, types.ObjectStorageClassStandard)
		}
	}
}
