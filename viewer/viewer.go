package viewer

// ViewerFunc renders a fetch result into a Viewer. err is the error returned
// by the corresponding Fetcher[T].Fetch call, if any; implementations should
// render an error view when err != nil rather than assuming data is valid.
type ViewerFunc[T any] func(data T, err error) Viewer

type Viewer interface {
	IsErrorView() bool
	View()
}

// FuncViewer adapts a plain side-effecting print function to the Viewer
// interface, for views that don't fit the table/error/compound shapes.
type FuncViewer func()

func (f FuncViewer) IsErrorView() bool { return false }
func (f FuncViewer) View()             { f() }

func NewTableViewer() *TableViewer {
	return &TableViewer{}
}

func NewErrorViewer() *ErrorViewer {
	return &ErrorViewer{}
}

func NewCompoundViewer() *CompoundViewer {
	return &CompoundViewer{}
}
