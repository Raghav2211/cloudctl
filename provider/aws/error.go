package aws

import (
	"cloudctl/viewer"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
)

type ErrorInfo struct {
	Err       error
	Meta      interface{}
	ErrorType viewer.ErrorType
}

func NewErrorInfo(err error, errorType viewer.ErrorType, meta interface{}) *ErrorInfo {
	return &ErrorInfo{
		Err:       err,
		Meta:      meta,
		ErrorType: errorType,
	}
}

// Error implements the error interface so *ErrorInfo can be returned
// directly from a Fetcher[T].Fetch call while still carrying its ErrorType
// severity for viewers to recover via errors.As.
func (e *ErrorInfo) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// ErrorView renders any error into an ErrorViewer, extracting the severity
// from an *ErrorInfo if err wraps one, defaulting to ERROR otherwise.
func ErrorView(err error) *viewer.ErrorViewer {
	errType := viewer.ERROR
	var info *ErrorInfo
	if errors.As(err, &info) {
		errType = info.ErrorType
	}
	ev := viewer.NewErrorViewer()
	ev.SetErrorType(errType)
	ev.SetErrorMessage(err.Error())
	return ev
}

// ErrorCategory classifies why an AWS call failed, independent of severity
// (viewer.ErrorType) — a NotFound can be an INFO, an AccessDenied is
// usually an ERROR, but both are categories a user can act on differently.
type ErrorCategory string

const (
	CategoryAuthentication ErrorCategory = "authentication"
	CategoryPermission     ErrorCategory = "permission"
	CategoryThrottling     ErrorCategory = "throttling"
	CategoryNotFound       ErrorCategory = "not_found"
	CategoryConnectivity   ErrorCategory = "connectivity"
	CategoryUnknown        ErrorCategory = "unknown"
)

// errorCodeCategories maps known AWS/smithy error codes to a category. Codes
// not listed here fall back to substring matching in categorize, then to
// CategoryUnknown — this table only needs the codes that substring matching
// can't reliably catch (e.g. "AuthFailure" doesn't contain "Auth" as a
// distinct word-safe substring check would want).
var errorCodeCategories = map[string]ErrorCategory{
	"UnrecognizedClientException": CategoryAuthentication,
	"InvalidClientTokenId":        CategoryAuthentication,
	"ExpiredToken":                CategoryAuthentication,
	"ExpiredTokenException":       CategoryAuthentication,
	"AuthFailure":                 CategoryAuthentication,
	"AccessDenied":                CategoryPermission,
	"AccessDeniedException":       CategoryPermission,
	"UnauthorizedOperation":       CategoryPermission,
	"ThrottlingException":         CategoryThrottling,
	"Throttling":                  CategoryThrottling,
	"RequestLimitExceeded":        CategoryThrottling,
	"TooManyRequestsException":    CategoryThrottling,
	"SlowDown":                    CategoryThrottling,
}

// categorize classifies a smithy API error's code into an ErrorCategory.
func categorize(code string) ErrorCategory {
	if c, ok := errorCodeCategories[code]; ok {
		return c
	}
	if strings.Contains(code, "NotFound") || strings.Contains(code, "NoSuch") {
		return CategoryNotFound
	}
	return CategoryUnknown
}

// categoryHint gives a short, actionable next step per category — the
// roadmap's "what can the user do next" requirement for error messages.
func categoryHint(c ErrorCategory) string {
	switch c {
	case CategoryAuthentication:
		return "credentials are missing or invalid — check --profile/--accessKey/--secretKey or AWS_PROFILE/AWS_ACCESS_KEY_ID"
	case CategoryPermission:
		return "the current identity lacks permission for this operation — check its IAM policy"
	case CategoryThrottling:
		return "AWS is rate-limiting requests — retry after a short delay"
	case CategoryNotFound:
		return "the requested resource does not exist in this account/region"
	default:
		return "an AWS API error occurred"
	}
}

// AWSError classifies err into an ErrorCategory (authentication, permission,
// throttling, not_found, connectivity, or unknown) and wraps it with a short
// actionable hint, preserving the original error in the chain via %w so
// errors.Is/errors.As against the underlying AWS error still work for every
// caller downstream (ADR-0017).
func AWSError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		category := categorize(apiErr.ErrorCode())
		return fmt.Errorf("[%s] %s (code:%s, message:%s): %w", category, categoryHint(category), apiErr.ErrorCode(), apiErr.ErrorMessage(), err)
	}
	return fmt.Errorf("[%s] %s: %w", CategoryConnectivity, "could not reach AWS — check network connectivity and region configuration", err)
}
