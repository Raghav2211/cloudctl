package viewer

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type ErrorType int

const (
	DEBUG ErrorType = iota
	WARN
	INFO
	ERROR
)

// errorTypeColors maps each severity to its adaptive color and banner text,
// part of cloudctl's design system (ADR-0019).
var errorTypeColors = map[ErrorType]lipgloss.AdaptiveColor{
	DEBUG: {Light: "#6E7781", Dark: "#8B949E"}, // muted gray
	WARN:  {Light: "#9A6700", Dark: "#D29922"}, // amber
	INFO:  {Light: "#5A56E0", Dark: "#8B87F7"}, // accent (same as headers — informational, not alarming)
	ERROR: {Light: "#CF222E", Dark: "#F85149"}, // red
}

var errorTypeBanner = map[ErrorType]string{
	DEBUG: "DEBUG!",
	WARN:  "WARNING!",
	INFO:  "INFO!",
	ERROR: "ERROR!",
}

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

// IsFailure is true only for ERROR severity — WARN/INFO/DEBUG banners (e.g.
// "no instances found", "bucket has more objects than max-keys") are
// informational, not failures, and must not turn into a non-zero exit code.
func (t *ErrorViewer) IsFailure() bool {
	return t.errorType == ERROR
}

func (e *ErrorViewer) View() {
	style := lipgloss.NewStyle().Bold(true).Foreground(errorTypeColors[e.errorType])
	fmt.Println()
	fmt.Println(style.Render(errorTypeBanner[e.errorType]))
	fmt.Println(style.Render(e.message))
}
