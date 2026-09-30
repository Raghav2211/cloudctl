package executor

import (
	"cloudctl/viewer"
	"context"
	"testing"
)

func TestRenderStructured_TableFormatIsNoop(t *testing.T) {
	SetOutputFormat(FormatTable)
	t.Cleanup(func() { SetOutputFormat(FormatTable) })

	if renderStructured(viewer.NewTableViewer()) {
		t.Error("expected FormatTable to never render structured output")
	}
}

func TestRenderStructured_NonStructurableViewerFallsBack(t *testing.T) {
	SetOutputFormat(FormatJSON)
	t.Cleanup(func() { SetOutputFormat(FormatTable) })

	if renderStructured(viewer.FuncViewer(func() {})) {
		t.Error("expected a non-Structurable Viewer to fall back to false")
	}
}

func TestRenderStructured_JSONAndYAML_RenderStructurableViewer(t *testing.T) {
	for _, format := range []OutputFormat{FormatJSON, FormatYAML} {
		SetOutputFormat(format)
		t.Cleanup(func() { SetOutputFormat(FormatTable) })

		tv := viewer.NewTableViewer().SetTitle("test")
		tv.AddHeader(viewer.Row{"Name"})
		tv.AddRow(viewer.Row{"x"})

		if !renderStructured(tv) {
			t.Errorf("format %s: expected a Structurable Viewer to render successfully", format)
		}
	}
}

func TestExecute_StructuredFormat_SkipsFooterAndSucceeds(t *testing.T) {
	SetOutputFormat(FormatJSON)
	t.Cleanup(func() { SetOutputFormat(FormatTable) })

	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{data: "ok"},
		Viewer: func(data string, err error) viewer.Viewer {
			return viewer.NewTableViewer()
		},
	}
	if err := exe.Execute(context.Background()); err != nil {
		t.Errorf("expected nil error in structured mode, got %v", err)
	}
}
