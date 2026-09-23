package executor

import (
	"cloudctl/viewer"
	"context"
	"errors"
	"testing"
)

type fakeFetcher struct {
	data string
	err  error
}

func (f fakeFetcher) Fetch(_ context.Context) (string, error) {
	return f.data, f.err
}

// TestExecute_Success_ReturnsNil confirms the happy path is unaffected.
func TestExecute_Success_ReturnsNil(t *testing.T) {
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{data: "ok"},
		Viewer: func(data string, err error) viewer.Viewer {
			return viewer.NewTableViewer()
		},
	}
	if err := exe.Execute(context.Background()); err != nil {
		t.Errorf("expected nil error on success, got %v", err)
	}
}

// TestExecute_ErrorSeverityFailure_ReturnsFetchErr is the ADR-0017
// regression test: a real Fetcher error rendered as an ERROR-severity view
// must now propagate as Execute's return value (previously always nil,
// silently exiting 0 regardless of failure).
func TestExecute_ErrorSeverityFailure_ReturnsFetchErr(t *testing.T) {
	boom := errors.New("boom")
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{err: boom},
		Viewer: func(data string, err error) viewer.Viewer {
			ev := viewer.NewErrorViewer()
			ev.SetErrorType(viewer.ERROR)
			ev.SetErrorMessage(err.Error())
			return ev
		},
	}
	got := exe.Execute(context.Background())
	if !errors.Is(got, boom) {
		t.Errorf("expected Execute to return the original fetch error, got %v", got)
	}
}

// TestExecute_InfoSeverity_StillReturnsNil confirms an INFO-severity view
// (e.g. "no instances found") does NOT cause a failing exit code — this is
// an empty/notable result, not a real failure, and must keep behaving
// exactly as before ADR-0017.
func TestExecute_InfoSeverity_StillReturnsNil(t *testing.T) {
	notFound := errors.New("no instances found")
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{err: notFound},
		Viewer: func(data string, err error) viewer.Viewer {
			ev := viewer.NewErrorViewer()
			ev.SetErrorType(viewer.INFO)
			ev.SetErrorMessage(err.Error())
			return ev
		},
	}
	if err := exe.Execute(context.Background()); err != nil {
		t.Errorf("expected nil error for an INFO-severity view, got %v", err)
	}
}

// TestExecute_WarnSeverity_StillReturnsNil mirrors the INFO case for WARN.
func TestExecute_WarnSeverity_StillReturnsNil(t *testing.T) {
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{err: errors.New("no object found with given prefix")},
		Viewer: func(data string, err error) viewer.Viewer {
			ev := viewer.NewErrorViewer()
			ev.SetErrorType(viewer.WARN)
			ev.SetErrorMessage(err.Error())
			return ev
		},
	}
	if err := exe.Execute(context.Background()); err != nil {
		t.Errorf("expected nil error for a WARN-severity view, got %v", err)
	}
}

// TestExecute_CompoundPartialFailure_ReturnsSentinel is the other half of
// the ADR-0017 fix: a multi-section view (ec2 def, ec2 explain, s3 impact)
// where the top-level Fetch succeeded (fetchErr == nil) but one sub-panel
// is an ERROR-severity ctlaws.ErrorView must still produce a non-nil error,
// since there's no single underlying Fetcher error to return.
func TestExecute_CompoundPartialFailure_ReturnsSentinel(t *testing.T) {
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{data: "partial"},
		Viewer: func(data string, err error) viewer.Viewer {
			compound := viewer.NewCompoundViewer()
			compound.AddViewer(viewer.NewTableViewer()) // succeeded section
			failed := viewer.NewErrorViewer()
			failed.SetErrorType(viewer.ERROR)
			failed.SetErrorMessage("volume lookup failed")
			compound.AddViewer(failed) // failed section
			return compound
		},
	}
	got := exe.Execute(context.Background())
	if !errors.Is(got, ErrCommandFailed) {
		t.Errorf("expected Execute to return ErrCommandFailed for a partial compound failure, got %v", got)
	}
}

// TestExecute_CompoundAllSuccess_ReturnsNil confirms a multi-section view
// with no failing sub-panel is unaffected.
func TestExecute_CompoundAllSuccess_ReturnsNil(t *testing.T) {
	exe := &CommandExecutor[string]{
		Fetcher: fakeFetcher{data: "ok"},
		Viewer: func(data string, err error) viewer.Viewer {
			compound := viewer.NewCompoundViewer()
			compound.AddViewer(viewer.NewTableViewer())
			compound.AddViewer(viewer.NewTableViewer())
			return compound
		},
	}
	if err := exe.Execute(context.Background()); err != nil {
		t.Errorf("expected nil error for an all-success compound, got %v", err)
	}
}
