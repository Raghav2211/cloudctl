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

// TestAWSError_APIError_PreservesChain is a regression test for a real bug
// found via architecture review (ADR-0017): the API-error branch used to
// reformat via fmt.Errorf with no %w verb, silently discarding the original
// error from the chain — errors.Is/errors.As against the underlying AWS
// error would fail for any caller downstream of AWSError.
func TestAWSError_APIError_PreservesChain(t *testing.T) {
	apiErr := &smithy.GenericAPIError{Code: "NoSuchBucket", Message: "The specified bucket does not exist"}

	got := AWSError(apiErr)

	if !errors.Is(got, apiErr) {
		t.Errorf("expected AWSError to preserve the original API error in its chain via %%w, got %q", got.Error())
	}
	var recovered smithy.APIError
	if !errors.As(got, &recovered) {
		t.Errorf("expected errors.As to recover the smithy.APIError from AWSError's result, got %q", got.Error())
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
