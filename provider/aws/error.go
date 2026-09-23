package aws

import (
	"cloudctl/viewer"
	"errors"
	"fmt"

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

func AWSError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return fmt.Errorf("[code:%s, message:%s]: %w", apiErr.ErrorCode(), apiErr.ErrorMessage(), err)
	}
	return fmt.Errorf("something wrong, need to find out what | actual err %w", err)
}
