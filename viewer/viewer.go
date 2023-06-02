package viewer

type ViewerFunc func(o interface{}) Viewer

type Viewer interface {
	IsErrorView() bool
	View()
}

func NewTableViewer() *TableViewer {
	return &TableViewer{}
}

func NewCompoundTableViewer() *CompoundTableViewer {
	return &CompoundTableViewer{}
}
