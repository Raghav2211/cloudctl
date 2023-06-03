package viewer

type CompoundViewer struct {
	tViewers   []*TableViewer
	errViewers []*ErrorViewer
}

func (v *CompoundViewer) AddErrorViewer(errViewer *ErrorViewer) *CompoundViewer {
	v.errViewers = append(v.errViewers, errViewer)
	return v
}

func (v *CompoundViewer) AddTableViewer(tViewer *TableViewer) *CompoundViewer {
	v.tViewers = append(v.tViewers, tViewer)
	return v
}
func (v *CompoundViewer) IsErrorView() bool {
	return len(v.errViewers) == 0
}

func (v *CompoundViewer) View() {
	for _, tViewer := range v.tViewers {
		tViewer.View()
	}
	for _, errViewer := range v.errViewers {
		errViewer.View()
	}
}
