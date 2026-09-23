package viewer

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
)

// PanelEntry is one labeled line in a Panel's entry list.
type PanelEntry struct {
	Label string
	Value string
}

// Panel renders a bordered, titled block for single-resource displays that
// don't fit TableViewer's many-rows-same-columns shape: either freeform
// prose (an AI summary) via SetBody, or label/value pairs (a resource's
// fields) via AddEntry. Reuses TableStyle and the same border/formatting
// helpers TableViewer uses, so panels and tables look visually consistent.
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
// Panels, not one, to avoid mixing a single-column prose row with two-column
// label/value rows in the same table.
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
	writer := table.NewWriter()
	writer.SetStyle(table.StyleDefault)
	applyBorderStyle(writer, p.style.BorderStyle)

	if p.title != "" {
		titleColor := color.New(p.style.TitleColor, color.Bold)
		writer.SetTitle(titleColor.Sprint(p.title))
	}

	if p.body != "" {
		writer.AppendRow(table.Row{p.body})
		writer.SetColumnConfigs([]table.ColumnConfig{{Number: 1, WidthMax: 100}})
	} else {
		labelColor := color.New(p.style.HeaderColor, color.Bold)
		for _, e := range p.entries {
			writer.AppendRow(table.Row{labelColor.Sprint(e.Label), e.Value})
		}
		writer.SetColumnConfigs([]table.ColumnConfig{{Number: 2, WidthMax: 90}})
	}

	fmt.Println(formatOutput(writer.Render()))
}
