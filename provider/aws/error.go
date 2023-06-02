package aws

import (
	"cloudctl/viewer"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
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

func AWSError(err error) error {
	if awserr, ok := err.(awserr.Error); ok {
		return fmt.Errorf("[code:%s , message:%s] ", awserr.Code(), awserr.Message())
	}
	return fmt.Errorf("somethig wrong, Need to find out what :) | actual err %w", err)
}
