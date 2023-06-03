package viewer

import (
	"fmt"

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
	writer.SetTitle(t.title)
	writer.SetAutoIndex(true)
	writer.AppendHeader(t.header)
	for _, row := range t.rows {
		writer.AppendRow(row)
	}
	fmt.Println(writer.Render())

}
