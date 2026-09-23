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

// IsFailure is true if any child is a real failure (ERROR severity) — e.g.
// one failed sub-section in an otherwise-successful multi-panel view like
// `ec2 def` or `ec2 explain` still needs to produce a non-zero exit code,
// even though the overall render (and IsErrorView, which only affects
// whether the "Time elapsed" footer prints) is unaffected.
func (v *CompoundViewer) IsFailure() bool {
	for _, viewer := range v.viewers {
		if viewer.IsFailure() {
			return true
		}
	}
	return false
}

func (v *CompoundViewer) View() {
	for _, viewer := range v.viewers {
		viewer.View()
	}
}
