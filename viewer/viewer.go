package viewer

type Viewer interface {
	View()
}

func NewTableViewer() *TableViewer {
	return &TableViewer{}
}

func NewCompoundTableViewer() *CompoundTableViewer {
	return &CompoundTableViewer{}
}
