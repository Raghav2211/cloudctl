package viewer

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	ltree "github.com/charmbracelet/lipgloss/tree"
)

// Tree renders hierarchical data (e.g. a VPC's subnets and route tables, or
// an EKS cluster's node groups) as a real tree instead of a flat table
// forcing a parent-ID column. Not used by any command yet — added in
// Track H so Track I's VPC/EKS commands have it from day one (ADR-0019).
type Tree struct {
	title string
	root  *ltree.Tree
	style TableStyle
}

// NewTree starts a tree rooted at label (e.g. a VPC ID).
func NewTree(label string) *Tree {
	return &Tree{root: ltree.Root(label), style: DefaultTableStyle()}
}

func (t *Tree) SetTitle(title string) *Tree {
	t.title = title
	return t
}

func (t *Tree) SetStyle(style TableStyle) *Tree {
	t.style = style
	return t
}

// Child adds a leaf or sub-tree under the root. Pass a *Tree (from a nested
// NewTree call) to build multiple levels, or a plain string for a leaf.
func (t *Tree) Child(child any) *Tree {
	if nested, ok := child.(*Tree); ok {
		t.root.Child(nested.root)
		return t
	}
	t.root.Child(child)
	return t
}

func (t *Tree) IsErrorView() bool { return false }
func (t *Tree) IsFailure() bool   { return false }

func (t *Tree) View() {
	rootStyle := lipgloss.NewStyle().Bold(true).Foreground(t.style.Accent)
	itemStyle := lipgloss.NewStyle()
	enumStyle := lipgloss.NewStyle().Foreground(t.style.Border).PaddingRight(1)
	t.root.RootStyle(rootStyle).ItemStyle(itemStyle).EnumeratorStyle(enumStyle)

	fmt.Println()
	if t.title != "" {
		fmt.Println(lipgloss.NewStyle().Bold(true).Render(t.title))
	}
	fmt.Println(t.root.String())
}
