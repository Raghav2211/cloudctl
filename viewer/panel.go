package viewer

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PanelEntry is one labeled line in a Panel's entry list.
type PanelEntry struct {
	Label string
	Value string
}

// Panel renders a bordered, titled block for single-resource displays that
// don't fit TableViewer's many-rows-same-columns shape: either freeform
// prose (an AI summary) via SetBody, or label/value pairs (a resource's
// fields) via AddEntry. Shares TableStyle with TableViewer so panels and
// tables look visually consistent (ADR-0019).
type Panel struct {
	title   string
	body    string
	entries []PanelEntry
	style   TableStyle
}

func NewPanel() *Panel {
	return &Panel{style: DefaultTableStyle()}
}

func (p *Panel) SetTitle(title string) *Panel {
	p.title = title
	return p
}

func (p *Panel) SetStyle(style TableStyle) *Panel {
	p.style = style
	return p
}

// SetBody sets freeform prose content (e.g. an AI summary). A Panel with a
// body renders that body only — pair body and entries via two separate
// Panels, not one, to avoid mixing a single-column prose block with
// two-column label/value rows in the same panel.
func (p *Panel) SetBody(body string) *Panel {
	p.body = body
	return p
}

func (p *Panel) AddEntry(label, value string) *Panel {
	p.entries = append(p.entries, PanelEntry{Label: label, Value: value})
	return p
}

func (p *Panel) IsErrorView() bool { return false }
func (p *Panel) IsFailure() bool   { return false }

func (p *Panel) View() {
	// Unlike TableViewer, a Panel's content (prose or label/value pairs) is
	// always better wrapped at some width than left unbounded — an AI
	// summary paragraph rendered as one giant unwrapped line is exactly
	// the kind of output this design is meant to fix. Prefer an explicit
	// MaxWidth, then the real terminal width, then a prose-friendly default.
	maxWidth := p.style.MaxWidth
	if maxWidth <= 0 {
		maxWidth = terminalWidth()
	}
	if maxWidth <= 0 {
		maxWidth = 100
	}
	contentMaxWidth := maxWidth - 6 // border + padding allowance

	var content string
	if p.body != "" {
		content = wrapText(p.body, contentMaxWidth)
	} else {
		labelStyle := lipgloss.NewStyle().Bold(true).Foreground(p.style.Accent)
		labelWidth := 0
		for _, e := range p.entries {
			if len(e.Label) > labelWidth {
				labelWidth = len(e.Label)
			}
		}
		valueMaxWidth := contentMaxWidth - labelWidth - 2
		if valueMaxWidth < 10 {
			valueMaxWidth = 10
		}
		rows := make([]string, 0, len(p.entries))
		for _, e := range p.entries {
			label := labelStyle.Width(labelWidth).Render(e.Label) // pad the (short, known-length) label column only
			value := wrapText(e.Value, valueMaxWidth)
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, label, "  ", value))
		}
		content = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.style.Border).
		Padding(0, 1)

	fmt.Println()
	if p.title != "" {
		fmt.Println(lipgloss.NewStyle().Bold(true).Render(p.title))
	}
	fmt.Println(borderStyle.Render(content))
}

// wrapText wraps s to width columns without padding shorter lines to fill
// it — unlike lipgloss's Style.Width(), which both wraps AND pads every
// line to exactly the target width, which would stretch a short message to
// fill the full panel width regardless of how little text there is.
//
// Wrapping happens at word boundaries where possible, but a single token
// longer than width (e.g. an IAM policy JSON blob, which has no spaces at
// all) is hard-broken into width-sized chunks — otherwise content with no
// natural break points would still render as one unbounded line, which was
// the original, most concrete complaint this design fixes.
func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	var lines []string
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			lines = append(lines, "")
			continue
		}
		var line string
		for _, word := range strings.Fields(paragraph) {
			if lipgloss.Width(word) > width {
				// Flush the current line, then hard-break the long token
				// into clean width-sized chunks; the final partial chunk
				// becomes the start of the next line.
				if line != "" {
					lines = append(lines, line)
					line = ""
				}
				for lipgloss.Width(word) > width {
					var head string
					head, word = splitByWidth(word, width)
					lines = append(lines, head)
				}
				line = word
				continue
			}
			candidate := word
			if line != "" {
				candidate = line + " " + word
			}
			if lipgloss.Width(candidate) > width && line != "" {
				lines = append(lines, line)
				line = word
			} else {
				line = candidate
			}
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// splitByWidth splits s into a prefix at most width columns wide and the
// remaining suffix.
func splitByWidth(s string, width int) (head, tail string) {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s, ""
	}
	runes := []rune(s)
	for i := range runes {
		if lipgloss.Width(string(runes[:i+1])) > width {
			return string(runes[:i]), string(runes[i:])
		}
	}
	return s, ""
}
