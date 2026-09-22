package viewer

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
)

type Row []interface{}

type embedError struct {
	err       error
	errorType ErrorType
}

type TableViewer struct {
	title      string
	header     table.Row
	rows       []table.Row
	embedError embedError
	style      TableStyle
}

type TableStyle struct {
	TitleColor     color.Attribute
	HeaderColor    color.Attribute
	BorderColor    color.Attribute
	AlternateRows  bool
	AlternateColor color.Attribute
	Compact        bool
	MaxWidth       int
	SortBy         int
	SortDescending bool
	BorderStyle    string // "rounded", "double", "thick", "minimal"
	ShowRowNumbers bool
	HeaderStyle    string // "bold", "underline", "italic"
}

func DefaultTableStyle() TableStyle {
	return TableStyle{
		TitleColor:     color.FgCyan,
		HeaderColor:    color.FgYellow,
		BorderColor:    color.FgBlue,
		AlternateRows:  true,
		AlternateColor: color.FgHiBlack,
		Compact:        false,
		MaxWidth:       0,  // 0 means no limit
		SortBy:         -1, // -1 means no sorting
		SortDescending: false,
		BorderStyle:    "rounded",
		ShowRowNumbers: true,
		HeaderStyle:    "bold",
	}
}

func (t *TableViewer) AddHeader(header Row) *TableViewer {
	headerr := table.Row{}
	for _, h := range header {
		headerr = append(headerr, h)
	}
	t.header = headerr
	return t
}

func (t *TableViewer) AddRow(row Row) *TableViewer {
	roww := table.Row{}
	for _, r := range row {
		roww = append(roww, r)
	}
	t.rows = append(t.rows, roww)
	return t
}

func (t *TableViewer) AddRows(rows []Row) *TableViewer {
	for _, roww := range rows {
		t.AddRow(roww)
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

func (t *TableViewer) SetError(err error, errorType ErrorType) *TableViewer {
	t.embedError = embedError{
		err:       err,
		errorType: errorType,
	}
	return t
}

func (t *TableViewer) IsErrorView() bool {
	return false
}

func (t *TableViewer) View() {
	writer := table.NewWriter()

	// Apply base styling
	writer.SetStyle(table.StyleDefault)

	// Apply custom border style
	applyBorderStyle(writer, t.style.BorderStyle)

	// Set title with enhanced styling
	if t.title != "" {
		titleColor := color.New(t.style.TitleColor, color.Bold)
		writer.SetTitle(titleColor.Sprint(t.title))
	}

	// Configure table options
	if t.style.ShowRowNumbers {
		writer.SetAutoIndex(true)
		writer.SetIndexColumn(1)
	}

	// Set compact mode with enhanced styling
	if t.style.Compact {
		writer.Style().Box = table.StyleBoxLight
		writer.Style().Options.SeparateRows = false
	}

	// Add header with enhanced styling
	if len(t.header) > 0 {
		headerColor := color.New(t.style.HeaderColor, color.Bold)
		coloredHeader := table.Row{}
		for _, h := range t.header {
			coloredHeader = append(coloredHeader, headerColor.Sprint(fmt.Sprintf("%v", h)))
		}
		writer.AppendHeader(coloredHeader)
	}

	// Add rows with enhanced styling
	for i, row := range t.rows {
		if t.style.AlternateRows && i%2 == 1 {
			coloredRow := table.Row{}
			rowColor := color.New(t.style.AlternateColor)
			for _, cell := range row {
				coloredRow = append(coloredRow, rowColor.Sprint(cell))
			}
			writer.AppendRow(coloredRow)
		} else {
			writer.AppendRow(row)
		}
	}

	// Apply width limit if specified
	if t.style.MaxWidth > 0 {
		writer.SetAllowedRowLength(t.style.MaxWidth)
	}

	// Sort if specified
	if t.style.SortBy >= 0 && t.style.SortBy < len(t.header) {
		writer.SortBy([]table.SortBy{
			{Number: t.style.SortBy + 1, Mode: table.Asc},
		})
		if t.style.SortDescending {
			writer.SortBy([]table.SortBy{
				{Number: t.style.SortBy + 1, Mode: table.Dsc},
			})
		}
	}

	// Render with custom formatting
	output := writer.Render()

	// Apply additional formatting
	output = formatOutput(output)

	fmt.Println(output)
}

// applyBorderStyle applies different border styles
func applyBorderStyle(writer table.Writer, style string) {
	switch style {
	case "rounded":
		writer.Style().Box = table.StyleBoxRounded
	case "double":
		writer.Style().Box = table.StyleBoxDouble
	case "thick":
		writer.Style().Box = table.StyleBoxDefault
	case "minimal":
		writer.Style().Box = table.StyleBoxLight
		writer.Style().Options.SeparateRows = false
		writer.Style().Options.SeparateColumns = false
	default:
		writer.Style().Box = table.StyleBoxDefault
	}
}

func formatOutput(output string) string {
	// Add some spacing for better readability
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		// Add a blank line before the table
		output = "\n" + output
	}
	return output
}
