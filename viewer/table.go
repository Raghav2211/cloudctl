package viewer

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

type Row []interface{}

type TableViewer struct {
	title  string
	header Row
	rows   []Row
	style  TableStyle
}

func (t *TableViewer) AddHeader(header Row) *TableViewer {
	t.header = header
	return t
}

func (t *TableViewer) AddRow(row Row) *TableViewer {
	t.rows = append(t.rows, row)
	return t
}

func (t *TableViewer) AddRows(rows []Row) *TableViewer {
	for _, row := range rows {
		t.AddRow(row)
	}
	return t
}

func (t *TableViewer) SetTitle(title string) *TableViewer {
	t.title = title
	return t
}

func (t *TableViewer) SetStyle(style TableStyle) *TableViewer {
	t.style = style
	return t
}

func (t *TableViewer) IsErrorView() bool {
	return false
}

func (t *TableViewer) IsFailure() bool {
	return false
}

func (t *TableViewer) View() {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(t.style.Accent).Padding(0, 1)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(t.style.Border)

	tbl := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return cellStyle
		})
	if t.style.MaxWidth > 0 {
		tbl = tbl.Width(t.style.MaxWidth)
	}

	headerRow := make([]string, 0, len(t.header)+1)
	if t.style.ShowRowNumbers {
		headerRow = append(headerRow, "#")
	}
	for _, h := range t.header {
		headerRow = append(headerRow, fmt.Sprintf("%v", h))
	}
	tbl.Headers(headerRow...)

	for i, row := range t.rows {
		strRow := make([]string, 0, len(row)+1)
		if t.style.ShowRowNumbers {
			strRow = append(strRow, fmt.Sprintf("%d", i+1))
		}
		for _, cell := range row {
			strRow = append(strRow, fmt.Sprintf("%v", cell))
		}
		tbl.Row(strRow...)
	}

	fmt.Println()
	if t.title != "" {
		fmt.Println(lipgloss.NewStyle().Bold(true).Render(t.title))
	}
	fmt.Println(tbl.String())
}
