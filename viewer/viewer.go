package viewer

type ViewerFunc func(o interface{}) Viewer

type Viewer interface {
	View()
}

func NewTableViewer() *TableViewer {
	return &TableViewer{}
}

func NewCompoundTableViewer() *CompoundTableViewer {
	return &CompoundTableViewer{}
}
