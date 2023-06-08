package viewer

type CompoundViewer struct {
	viewers []Viewer
}

func (v *CompoundViewer) AddViewer(viewer Viewer) *CompoundViewer {
	v.viewers = append(v.viewers, viewer)
	return v
}
func (v *CompoundViewer) AddViewers(viewers []Viewer) *CompoundViewer {
	for _, viewer := range viewers {
		v.AddViewer(viewer)
	}
	return v
}

func (v *CompoundViewer) IsErrorView() bool {
	return len(v.viewers) == 1 && v.viewers[len(v.viewers)-1].IsErrorView()
}

func (v *CompoundViewer) View() {
	for _, viewer := range v.viewers {
		viewer.View()
	}
}
