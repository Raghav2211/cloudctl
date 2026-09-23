package viewer

// ViewerFunc renders a fetch result into a Viewer. err is the error returned
// by the corresponding Fetcher[T].Fetch call, if any; implementations should
// render an error view when err != nil rather than assuming data is valid.
type ViewerFunc[T any] func(data T, err error) Viewer

type Viewer interface {
	IsErrorView() bool
	// IsFailure reports whether this view represents a real failure the
	// caller should be told about via a non-zero process exit code — true
	// only for ERROR-severity content, never WARN/INFO/DEBUG. This is a
	// separate signal from IsErrorView, which only controls rendering
	// details (e.g. whether the "Time elapsed" footer prints) and stays
	// true for every severity, exactly as before.
	IsFailure() bool
	View()
}

// FuncViewer adapts a plain side-effecting print function to the Viewer
// interface, for views that don't fit the table/error/compound shapes.
type FuncViewer func()

func (f FuncViewer) IsErrorView() bool { return false }
func (f FuncViewer) IsFailure() bool   { return false }
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
