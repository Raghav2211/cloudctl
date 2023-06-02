package viewer

import (
	"github.com/fatih/color"
)

type ErrorType color.Attribute

const (
	WARN  ErrorType = ErrorType(color.FgYellow)
	INFO  ErrorType = ErrorType(color.FgBlue)
	ERROR ErrorType = ErrorType(color.FgRed)
)

type ErrorViewer struct {
	message   string
	meta      interface{}
	errorType ErrorType
}

func (e *ErrorViewer) SetErrorMessage(message string) *ErrorViewer {
	e.message = message
	return e
}

func (e *ErrorViewer) SetErrorMeta(meta interface{}) *ErrorViewer {
	e.meta = meta
	return e
}

func (e *ErrorViewer) SetErrorType(errorType ErrorType) *ErrorViewer {
	e.errorType = errorType
	return e
}

func (t *ErrorViewer) IsErrorView() bool {
	return true
}
func (e *ErrorViewer) View() {
	black := color.New(color.Attribute(e.errorType))
	boldBlack := black.Add(color.Bold)
	boldBlack.Println("Error found: ", e.message, " meta: ", e.meta)
}
