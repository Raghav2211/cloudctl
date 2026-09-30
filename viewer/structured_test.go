package viewer

import (
	"strings"
	"testing"
)

func TestStructuredJSON_TableViewer(t *testing.T) {
	tv := NewTableViewer().SetTitle("Buckets")
	tv.AddHeader(Row{"Name", "Region"})
	tv.AddRow(Row{"my-bucket", "eu-west-1"})

	got := StructuredJSON(tv)
	if !strings.Contains(got, `"my-bucket"`) || !strings.Contains(got, `"Buckets"`) {
		t.Errorf("expected JSON containing bucket name and title, got %q", got)
	}
}

func TestStructuredJSON_NonStructurable(t *testing.T) {
	if got := StructuredJSON(nil); got != "" {
		t.Errorf("expected empty string for a non-Structurable Viewer, got %q", got)
	}
}

func TestTableViewer_Structured(t *testing.T) {
	tv := NewTableViewer().SetTitle("Buckets")
	tv.AddHeader(Row{"Name", "Region"})
	tv.AddRow(Row{"my-bucket", "eu-west-1"})

	got, ok := tv.Structured().(structuredTable)
	if !ok {
		t.Fatalf("expected structuredTable, got %T", tv.Structured())
	}
	if got.Title != "Buckets" {
		t.Errorf("expected title 'Buckets', got %q", got.Title)
	}
	if len(got.Headers) != 2 || got.Headers[0] != "Name" {
		t.Errorf("expected headers [Name Region], got %v", got.Headers)
	}
	if len(got.Rows) != 1 || got.Rows[0][0] != "my-bucket" {
		t.Errorf("expected 1 row [my-bucket eu-west-1], got %v", got.Rows)
	}
}

func TestPanel_Structured(t *testing.T) {
	p := NewPanel().SetTitle("Health")
	p.AddEntry("Status", "healthy")

	got, ok := p.Structured().(structuredPanel)
	if !ok {
		t.Fatalf("expected structuredPanel, got %T", p.Structured())
	}
	if got.Title != "Health" {
		t.Errorf("expected title 'Health', got %q", got.Title)
	}
	if got.Entries["Status"] != "healthy" {
		t.Errorf("expected entries[Status]=healthy, got %v", got.Entries)
	}
}

func TestPanel_Structured_BodyOnly(t *testing.T) {
	p := NewPanel().SetBody("some prose")

	got := p.Structured().(structuredPanel)
	if got.Body != "some prose" {
		t.Errorf("expected body 'some prose', got %q", got.Body)
	}
	if got.Entries != nil {
		t.Errorf("expected no entries for a body-only panel, got %v", got.Entries)
	}
}

func TestCompoundViewer_Structured_FlattensChildren(t *testing.T) {
	compound := NewCompoundViewer()
	compound.AddViewer(NewPanel().SetTitle("A").SetBody("a"))
	compound.AddViewer(NewPanel().SetTitle("B").SetBody("b"))

	got, ok := compound.Structured().([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", compound.Structured())
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 flattened children, got %d", len(got))
	}
}

func TestErrorViewer_Structured(t *testing.T) {
	e := NewErrorViewer().SetErrorMessage("boom").SetErrorType(ERROR)

	got, ok := e.Structured().(structuredError)
	if !ok {
		t.Fatalf("expected structuredError, got %T", e.Structured())
	}
	if got.Error != "boom" {
		t.Errorf("expected error 'boom', got %q", got.Error)
	}
}

func TestTree_Structured_NestedChildren(t *testing.T) {
	sub := NewTree("subnet-1").SetTitle("Subnet")
	sub.Child("nat-gateway nat-1")
	tree := NewTree("vpc-1").SetTitle("VPC Topology")
	tree.Child(sub)
	tree.Child("internet-gateway igw-1")

	got, ok := tree.Structured().(structuredTree)
	if !ok {
		t.Fatalf("expected structuredTree, got %T", tree.Structured())
	}
	if got.Root != "vpc-1" {
		t.Errorf("expected root 'vpc-1', got %q", got.Root)
	}
	if len(got.Children) != 2 {
		t.Fatalf("expected 2 children, got %d: %+v", len(got.Children), got.Children)
	}
	nestedChild, ok := got.Children[0].(structuredTree)
	if !ok {
		t.Fatalf("expected the first child to be a nested structuredTree, got %T", got.Children[0])
	}
	if nestedChild.Root != "subnet-1" || len(nestedChild.Children) != 1 {
		t.Errorf("expected the nested tree to carry its own child, got %+v", nestedChild)
	}
}
