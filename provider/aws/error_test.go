package aws

import (
	"errors"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
)

func TestAWSError_APIError_DistinctCodeAndMessage(t *testing.T) {
	apiErr := &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "The specified bucket does not exist"}

	got := AWSError(apiErr)

	if !strings.Contains(got.Error(), "NoSuchBucket") {
		t.Errorf("expected error to contain the code %q, got %q", "NoSuchBucket", got.Error())
	}
	if !strings.Contains(got.Error(), "The specified bucket does not exist") {
		t.Errorf("expected error to contain the message, got %q", got.Error())
	}
}

func TestAWSError_NonAPIError_FallsBackToWrapping(t *testing.T) {
	base := errors.New("connection reset")
	got := AWSError(base)

	if !errors.Is(got, base) {
		t.Errorf("expected the fallback error to wrap the original error via %%w, got %q", got.Error())
	}
}

func TestErrorInfo_ImplementsError(t *testing.T) {
	var err error = NewErrorInfo(errors.New("boom"), 0, nil)
	if err.Error() != "boom" {
		t.Errorf("expected *ErrorInfo.Error() to delegate to the wrapped error, got %q", err.Error())
	}
}
