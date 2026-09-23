package viewer

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// TableStyle is cloudctl's design system (ADR-0019): one accent color for
// headers/titles, a neutral border, and no alternating-row background —
// content renders in the terminal's own default foreground so nothing
// competes with it. Colors adapt automatically to light/dark terminals via
// lipgloss.AdaptiveColor, rather than a fixed guess.
type TableStyle struct {
	Accent lipgloss.AdaptiveColor // header row, panel/table titles
	Border lipgloss.AdaptiveColor
	Muted  lipgloss.AdaptiveColor // secondary/placeholder text (unused today, available for future fields)

	// MaxWidth caps table/panel width. 0 (the default) means natural,
	// uncapped sizing — the right choice for typical multi-column ID
	// tables, where forcing a fixed width causes lipgloss/table's column
	// redistribution to wrap every column instead of just the wide one.
	// Call sites with a genuinely oversized single field (e.g. an IAM
	// policy JSON blob, a long joined action list) should set MaxWidth
	// explicitly so that specific content wraps instead of stretching the
	// table to hundreds of columns wide.
	MaxWidth       int
	ShowRowNumbers bool
}

// DefaultTableStyle returns cloudctl's committed palette. See ADR-0019.
func DefaultTableStyle() TableStyle {
	return TableStyle{
		Accent:         lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#8B87F7"},
		Border:         lipgloss.AdaptiveColor{Light: "#D0D0D0", Dark: "#4A4A4A"},
		Muted:          lipgloss.AdaptiveColor{Light: "#6E7781", Dark: "#8B949E"},
		MaxWidth:       0,
		ShowRowNumbers: true,
	}
}

// terminalWidth returns the real terminal width when stdout is a TTY, or 0
// when it can't be detected (piped/redirected output) — callers that want a
// width cap only when there's a real terminal to cap it to should treat 0
// as "no cap."
func terminalWidth() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	return 0
}
