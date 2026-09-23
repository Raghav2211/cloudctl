package viewer

import "testing"

func errorViewerOfType(t ErrorType) *ErrorViewer {
	ev := NewErrorViewer()
	ev.SetErrorType(t)
	ev.SetErrorMessage("x")
	return ev
}

func TestCompoundViewer_IsFailure_TrueIfAnyChildIsErrorSeverity(t *testing.T) {
	c := NewCompoundViewer()
	c.AddViewer(NewTableViewer())
	c.AddViewer(errorViewerOfType(ERROR))
	if !c.IsFailure() {
		t.Error("expected IsFailure to be true when any child is ERROR severity")
	}
}

func TestCompoundViewer_IsFailure_FalseForInfoAndWarnOnly(t *testing.T) {
	c := NewCompoundViewer()
	c.AddViewer(NewTableViewer())
	c.AddViewer(errorViewerOfType(INFO))
	c.AddViewer(errorViewerOfType(WARN))
	if c.IsFailure() {
		t.Error("expected IsFailure to be false when no child is ERROR severity")
	}
}

func TestCompoundViewer_IsFailure_FalseWhenAllSucceed(t *testing.T) {
	c := NewCompoundViewer()
	c.AddViewer(NewTableViewer())
	c.AddViewer(NewPanel())
	if c.IsFailure() {
		t.Error("expected IsFailure to be false for an all-success compound")
	}
}

func TestCompoundViewer_IsFailure_FalseWhenEmpty(t *testing.T) {
	c := NewCompoundViewer()
	if c.IsFailure() {
		t.Error("expected IsFailure to be false for an empty compound")
	}
}

// TestCompoundViewer_IsErrorView_UnchangedBehavior confirms this refactor
// did not touch IsErrorView's existing single-child-only semantics, which
// still controls only whether the executor's "Time elapsed" footer prints —
// a separate, unrelated concern from IsFailure's exit-code decision.
func TestCompoundViewer_IsErrorView_UnchangedBehavior(t *testing.T) {
	single := NewCompoundViewer()
	single.AddViewer(errorViewerOfType(ERROR))
	if !single.IsErrorView() {
		t.Error("expected IsErrorView true for a single all-error child, as before")
	}

	multi := NewCompoundViewer()
	multi.AddViewer(NewTableViewer())
	multi.AddViewer(errorViewerOfType(ERROR))
	if multi.IsErrorView() {
		t.Error("expected IsErrorView false for a multi-child compound, as before (only IsFailure changed)")
	}
}
