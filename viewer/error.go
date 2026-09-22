package viewer

import (
	"github.com/fatih/color"
)

type ErrorType color.Attribute

const (
	DEBUG ErrorType = ErrorType(color.FgBlue)
	WARN  ErrorType = ErrorType(color.FgYellow)
	INFO  ErrorType = ErrorType(color.FgBlue)
	ERROR ErrorType = ErrorType(color.FgRed)
)

type ErrorViewer struct {
	message   string
	errorType ErrorType
}

func (e *ErrorViewer) SetErrorMessage(message string) *ErrorViewer {
	e.message = message
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
	boldColor := black.Add(color.Bold)
	if e.errorType == WARN {
		boldColor.Println("WARNING!")
	} else if e.errorType == ERROR {
		boldColor.Println("ERROR!")
	} else if e.errorType == DEBUG {
		boldColor.Println("DEBUG!")
	} else {
		boldColor.Println("INFO!")
	}
	boldColor.Println(e.message)
}
