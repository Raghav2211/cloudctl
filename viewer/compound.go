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

func (v *CompoundViewer) AddViewer(viewer Viewer) *CompoundViewer {
	if tviewer, ok := viewer.(*TableViewer); ok {
		v.AddTableViewer(tviewer)
	}
	if eViewer, ok := viewer.(*ErrorViewer); ok {
		v.AddErrorViewer(eViewer)
	}
	return v
}
func (v *CompoundViewer) AddViewers(viewers []Viewer) *CompoundViewer {
	for _, viewer := range viewers {
		v.AddViewer(viewer)
	}
	return v
}

func (v *CompoundViewer) IsErrorView() bool {
	return len(v.errViewers) == 0 && len(v.tViewers) == 0
}

func (v *CompoundViewer) View() {
	for _, tViewer := range v.tViewers {
		tViewer.View()
	}
	for _, errViewer := range v.errViewers {
		errViewer.View()
	}
}
