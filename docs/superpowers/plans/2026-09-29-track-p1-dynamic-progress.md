# Track P.1 — Dynamic Progress Messages (Pattern + EC2/RDS) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** replace the frozen `"Fetching..."` spinner text with messages
naming the actual operation in progress, for the two representative
service packages (EC2, RDS) the roadmap names as where to establish the
pattern first.

**Architecture:** `viewer.SetProgress(ctx, message)` already exists and
already updates a running spinner's text — nothing about that changes.
This plan adds exactly one new primitive, `viewer.WithNestedProgress`,
which decides at runtime whether to update an already-running spinner
(the normal CLI path, wrapped by `executor.CommandExecutor.Execute`) or
create its own (the `investigate` agent's path, which calls a service's
`Fetch` directly and bypasses that wrapper — confirmed via
`ec2.InvestigateInstanceDef`/`rds.InvestigateDBDef`, which call
`exec.Fetcher.Fetch(ctx)` directly rather than `exec.Execute(ctx)`). Every
other `SetProgress` call added in this plan is a plain, unconditional call
— safe to make at any AWS-call site since it's a documented no-op when no
spinner is present in `ctx`.

**Tech Stack:** Go, `github.com/briandowns/spinner` (already a dependency,
via `viewer/spinner.go`).

**Spec:** `/Users/raghav.joshi/.claude/plans/cloudctl-cost-aws-gives-quizzical-puzzle.md`
(Track P of the seven-track cost-visibility roadmap — this plan covers
Track P.1 only: the pattern itself, plus EC2 and RDS. The roadmap's
"Representative files" list names these same two packages and explicitly
defers the rest to later P.2/P.3 sessions).

## Global Constraints

- No existing exported signature changes (`WithSpinner`,
  `WithProgressSpinner`, `SetProgress` stay exactly as they are) — only a
  new function, `WithNestedProgress`, is added.
- A progress message must name the actual AWS operation (e.g. "Calling
  RDS DescribeDBInstances..."), never a generic restatement of
  "fetching."
- `SetProgress` calls added at AWS-call sites must never change control
  flow or error handling — they are side-effect-only and safe to call
  unconditionally.
- The `investigate` agent's tool-call path (which calls `Fetch` directly,
  bypassing `CommandExecutor.Execute`) must continue to show a spinner
  during AI narration exactly as it does today — this plan must not
  regress that path in the course of fixing the CLI path's overlapping
  spinner.

## Review Focus

- **The `investigate` agent's `ec2_def`/`rds_def` tools call `Fetch`
  directly, with no outer `WithProgressSpinner`** — `WithNestedProgress`
  must still show a spinner here (its fallback branch), not silently show
  nothing. Covered by `TestWithNestedProgress_FallsBackToOwnSpinnerWhenNoneInContext`.
- **The normal `ctl ec2 def`/`ctl rds def` CLI path already has an outer
  spinner from `executor.go`** — `WithNestedProgress` must update it, not
  start a second, visually-overlapping one. Covered by
  `TestWithNestedProgress_UpdatesExistingSpinnerInsteadOfCreatingNew`.
- **Existing fetcher tests construct fakes and call `Fetch` with
  `context.Background()`** (no terminal, no spinner) — every new
  `SetProgress`/`WithNestedProgress` call added to `fetcher.go` must be a
  safe no-op there, or every existing test in
  `provider/aws/services/rds` and `provider/aws/services/ec2` breaks.
  Covered by re-running the full existing suites for both packages after
  each change (Tasks 2 and 3, Step 3).
- **`dbDefinitionFetcher.Fetch`'s instance→cluster fallback** (an
  instance-not-found result falls through to a cluster lookup) must keep
  working with a `SetProgress` call inserted before each of the two
  lookups — this is a control-flow-adjacent change, not a pure addition,
  so it's worth re-confirming explicitly rather than assuming "just
  inserting a line" is risk-free. Covered by re-running
  `TestSummaryFetcher`-style existing tests for `dbDefinitionFetcher` in
  Task 2.
- **Paginated calls (`fetchInstanceList`, `dbListFetcher.Fetch`'s two
  paginators) should get one `SetProgress` call before the loop starts,
  not one per page** — spamming a message on every page would be worse
  than the static text it replaces. Enforced by code review of Tasks 2/3,
  not a unit test (there's no existing test fixture with enough pages to
  make a per-page call observable).

---

### Task 1: Add `viewer.WithNestedProgress`

**Files:**
- Modify: `cloudctl/viewer/spinner.go`
- Test: `cloudctl/viewer/spinner_test.go`

**Interfaces:**
- Produces: `viewer.WithNestedProgress[T any](ctx context.Context, message string, fn func() (T, error)) (T, error)` — used by Tasks 2 and 3.

- [ ] **Step 1: Write the failing tests**

Add to `cloudctl/viewer/spinner_test.go` (add `"github.com/briandowns/spinner"` to the import block):

```go
func TestWithNestedProgress_UpdatesExistingSpinnerInsteadOfCreatingNew(t *testing.T) {
	s := &spinner.Spinner{}
	ctx := context.WithValue(context.Background(), spinnerCtxKey{}, s)

	got, err := WithNestedProgress(ctx, "narrating...", func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
	if s.Suffix != " narrating..." {
		t.Errorf("expected the existing spinner's Suffix to be updated to the nested message, got %q", s.Suffix)
	}
}

func TestWithNestedProgress_FallsBackToOwnSpinnerWhenNoneInContext(t *testing.T) {
	got, err := WithNestedProgress(context.Background(), "narrating...", func() (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ok" {
		t.Errorf("expected %q, got %q", "ok", got)
	}
}

func TestWithNestedProgress_PropagatesError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := WithNestedProgress(context.Background(), "narrating...", func() (string, error) {
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected the fn's error to propagate, got %v", err)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./viewer/... -run TestWithNestedProgress -v`
Expected: build failure — `WithNestedProgress` doesn't exist yet.

- [ ] **Step 3: Add `WithNestedProgress` to `cloudctl/viewer/spinner.go`**

Add after `SetProgress`:

```go
// WithNestedProgress runs fn under progress reporting appropriate to
// whether ctx already carries a running spinner from an enclosing
// WithProgressSpinner call. If so, it just updates that spinner's message
// (via SetProgress) and calls fn directly — avoiding a second,
// visually-overlapping spinner. If ctx carries no spinner (e.g. a caller
// that invokes a Fetcher directly, bypassing CommandExecutor.Execute —
// see ec2.InvestigateInstanceDef, which the investigation agent's tools
// use), it falls back to WithSpinner's own dedicated spinner so the
// operation still shows a progress indicator on its own.
func WithNestedProgress[T any](ctx context.Context, message string, fn func() (T, error)) (T, error) {
	if _, ok := ctx.Value(spinnerCtxKey{}).(*spinner.Spinner); ok {
		SetProgress(ctx, message)
		return fn()
	}
	return WithSpinner(message, fn)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./viewer/... -v`
Expected: PASS for every test in the package, including the pre-existing
`TestWithProgressSpinner_ReturnsFnResult`,
`TestWithProgressSpinner_PropagatesError`, and
`TestSetProgress_NoSpinnerInContext` — confirming no regression — plus the
three new tests from Step 1.

- [ ] **Step 5: Commit**

```bash
git add viewer/spinner.go viewer/spinner_test.go
git commit -m "feat(viewer): add WithNestedProgress to avoid overlapping spinners"
```

---

### Task 2: Wire dynamic progress into RDS's fetcher

**Files:**
- Modify: `cloudctl/provider/aws/services/rds/fetcher.go`

**Interfaces:**
- Consumes: `viewer.SetProgress(ctx, message)` (existing);
  `viewer.WithNestedProgress` (Task 1).
- Produces: no new exported symbols — `dbListFetcher.Fetch`,
  `dbDefinitionFetcher.Fetch`, and `(*dbDefinition).applyAINarration` keep
  their exact existing signatures; only their internal progress reporting
  changes.

- [ ] **Step 1: Add `"fmt"` to the import block**

In `cloudctl/provider/aws/services/rds/fetcher.go`, change:

```go
import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"errors"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)
```

to:

```go
import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
)
```

- [ ] **Step 2: Add progress messages to `dbListFetcher.Fetch`**

Change:

```go
func (f dbListFetcher) Fetch(ctx context.Context) (*dbListOutput, error) {
	var items []*dbSummary

	instPaginator := rds.NewDescribeDBInstancesPaginator(f.client, &rds.DescribeDBInstancesInput{})
	for instPaginator.HasMorePages() {
		page, err := instPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, inst := range page.DBInstances {
			items = append(items, newDBSummaryFromInstance(inst))
		}
	}

	clusterPaginator := rds.NewDescribeDBClustersPaginator(f.client, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, c := range page.DBClusters {
			items = append(items, newDBSummaryFromCluster(c))
		}
	}
```

to:

```go
func (f dbListFetcher) Fetch(ctx context.Context) (*dbListOutput, error) {
	var items []*dbSummary

	viewer.SetProgress(ctx, "Calling RDS DescribeDBInstances...")
	instPaginator := rds.NewDescribeDBInstancesPaginator(f.client, &rds.DescribeDBInstancesInput{})
	for instPaginator.HasMorePages() {
		page, err := instPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, inst := range page.DBInstances {
			items = append(items, newDBSummaryFromInstance(inst))
		}
	}

	viewer.SetProgress(ctx, "Calling RDS DescribeDBClusters...")
	clusterPaginator := rds.NewDescribeDBClustersPaginator(f.client, &rds.DescribeDBClustersInput{})
	for clusterPaginator.HasMorePages() {
		page, err := clusterPaginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		for _, c := range page.DBClusters {
			items = append(items, newDBSummaryFromCluster(c))
		}
	}
```

(Only the two `viewer.SetProgress` lines are new — everything else in the
function is unchanged.)

- [ ] **Step 3: Add progress messages to `dbDefinitionFetcher.Fetch`**

Change:

```go
func (f dbDefinitionFetcher) Fetch(ctx context.Context) (*dbDefinition, error) {
	instOut, err := f.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{DBInstanceIdentifier: &f.identifier})
	if err == nil && len(instOut.DBInstances) > 0 {
		def := newDBDefinitionFromInstance(instOut.DBInstances[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	clusterOut, err := f.client.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{DBClusterIdentifier: &f.identifier})
	if err == nil && len(clusterOut.DBClusters) > 0 {
		def := newDBDefinitionFromCluster(clusterOut.DBClusters[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	return nil, ctlaws.NewErrorInfo(DatabaseNotFound(f.identifier), viewer.INFO, nil)
}
```

to:

```go
func (f dbDefinitionFetcher) Fetch(ctx context.Context) (*dbDefinition, error) {
	viewer.SetProgress(ctx, fmt.Sprintf("Calling RDS DescribeDBInstances for %s...", f.identifier))
	instOut, err := f.client.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{DBInstanceIdentifier: &f.identifier})
	if err == nil && len(instOut.DBInstances) > 0 {
		def := newDBDefinitionFromInstance(instOut.DBInstances[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	viewer.SetProgress(ctx, fmt.Sprintf("Calling RDS DescribeDBClusters for %s...", f.identifier))
	clusterOut, err := f.client.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{DBClusterIdentifier: &f.identifier})
	if err == nil && len(clusterOut.DBClusters) > 0 {
		def := newDBDefinitionFromCluster(clusterOut.DBClusters[0])
		def.applyAINarration(ctx, ai.NewClientFromEnv(), dbDefinitionEvidence(def))
		return def, nil
	}
	if err != nil && !isNotFound(err) {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	return nil, ctlaws.NewErrorInfo(DatabaseNotFound(f.identifier), viewer.INFO, nil)
}
```

- [ ] **Step 4: Replace the nested spinner in `applyAINarration`**

Change:

```go
	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})
```

to:

```go
	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithNestedProgress(ctx, "Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})
```

(Only `viewer.WithSpinner` → `viewer.WithNestedProgress` changes on that
one line — the function body passed to it is unchanged.)

- [ ] **Step 5: Run the existing RDS test suite to confirm no regression**

Run: `go test ./provider/aws/services/rds/... -v`
Expected: PASS for every existing test in the package — none of them
inspect spinner output, so adding `SetProgress`/`WithNestedProgress` calls
must not change any assertion's outcome. This package has no fetcher-level
tests to extend with new assertions (per the Review Focus note, spinner
text isn't observable without a TTY), so this step is the verification for
Steps 2-4.

- [ ] **Step 6: Build the whole module**

Run: `go build ./...`
Expected: succeeds — confirms the `"fmt"` import is actually used and
nothing else broke.

- [ ] **Step 7: Commit**

```bash
git add provider/aws/services/rds/fetcher.go
git commit -m "feat(rds): show real progress messages instead of a static spinner"
```

---

### Task 3: Wire dynamic progress into EC2's fetcher

**Files:**
- Modify: `cloudctl/provider/aws/services/ec2/fetcher.go`

**Interfaces:**
- Consumes: `viewer.SetProgress(ctx, message)` (existing);
  `viewer.WithNestedProgress` (Task 1).
- Produces: no new exported symbols — `instanceListFetcher.Fetch`,
  `instanceDefinitionFetcher.Fetch`, `statisticsFetcher.Fetch`,
  `fetchInstanceList`, `fetchInstanceDefinition`, and
  `(*instanceDefinition).applyAINarration` keep their exact existing
  signatures.

- [ ] **Step 1: Add a progress message to `fetchInstanceList`**

This helper is called by both `instanceListFetcher.Fetch` and
`statisticsFetcher.Fetch`, so one change here covers both callers. Change:

```go
func fetchInstanceList(ctx context.Context, client ec2.DescribeInstancesAPIClient, instanceListFilter InstanceListFilter) ([]types.Instance, error) {
	instances := []types.Instance{}
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: instanceListFilter.requestFilters(),
	})
```

to:

```go
func fetchInstanceList(ctx context.Context, client ec2.DescribeInstancesAPIClient, instanceListFilter InstanceListFilter) ([]types.Instance, error) {
	viewer.SetProgress(ctx, "Calling EC2 DescribeInstances...")
	instances := []types.Instance{}
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: instanceListFilter.requestFilters(),
	})
```

- [ ] **Step 2: Add progress messages to `statisticsFetcher.Fetch`**

Change:

```go
func (f statisticsFetcher) Fetch(ctx context.Context) (*instanceStatisticsListOutput, error) {
	runningFilter := *NewInstanceFilter(WithInstanceStates([]string{"running"}))
	instances, err := fetchInstanceList(ctx, f.client, runningFilter)
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(instances) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}

	currentTime := time.Now()
	startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()-2, 0, 0, 0, 0, currentTime.Location())

	g, gCtx := errgroup.WithContext(ctx)
```

to:

```go
func (f statisticsFetcher) Fetch(ctx context.Context) (*instanceStatisticsListOutput, error) {
	runningFilter := *NewInstanceFilter(WithInstanceStates([]string{"running"}))
	instances, err := fetchInstanceList(ctx, f.client, runningFilter)
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(instances) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}

	viewer.SetProgress(ctx, fmt.Sprintf("Fetching CloudWatch CPU statistics for %d running instance(s)...", len(instances)))
	currentTime := time.Now()
	startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()-2, 0, 0, 0, 0, currentTime.Location())

	g, gCtx := errgroup.WithContext(ctx)
```

(`fmt` is already imported in this file.)

- [ ] **Step 3: Add progress messages to `fetchInstanceDefinition`**

Change:

```go
func fetchInstanceDefinition(ctx context.Context, instanceId *string, tz *ctltime.Timezone, client *ec2.Client) (*instanceDefinition, error) {
	definition := newInstanceDefinition()
	networkinterfaces := []*instanceNetworkinterface{}

	data, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{*instanceId}})
```

to:

```go
func fetchInstanceDefinition(ctx context.Context, instanceId *string, tz *ctltime.Timezone, client *ec2.Client) (*instanceDefinition, error) {
	definition := newInstanceDefinition()
	networkinterfaces := []*instanceNetworkinterface{}

	viewer.SetProgress(ctx, fmt.Sprintf("Calling EC2 DescribeInstances for %s...", *instanceId))
	data, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{*instanceId}})
```

And change:

```go
	wg := new(sync.WaitGroup)
	if instance.BlockDeviceMappings != nil {
```

to:

```go
	viewer.SetProgress(ctx, "Fetching volume and network interface details...")
	wg := new(sync.WaitGroup)
	if instance.BlockDeviceMappings != nil {
```

(This second message only makes sense to show if at least one of the two
`if` blocks below it will actually run; that's fine to set unconditionally
here since it's immediately followed by the two `if instance.X != nil`
checks — if neither is true, the message is simply overwritten by
whatever the next stage sets, with no user-visible effect since nothing
slow happened in between.)

- [ ] **Step 4: Replace the nested spinner in `applyAINarration`**

Change:

```go
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
```

to:

```go
	result, _ := viewer.WithNestedProgress(ctx, "Generating AI summary and recommendations...", func() (narration, error) {
```

(Only this one line changes — the function literal's body is unchanged.)

- [ ] **Step 5: Run the existing EC2 test suite to confirm no regression**

Run: `go test ./provider/aws/services/ec2/... -v`
Expected: PASS for every existing test in the package (same reasoning as
Task 2, Step 5 — no existing test inspects spinner output).

- [ ] **Step 6: Run the full module build and test suite**

Run: `go build ./... && go test ./...`
Expected: everything builds and passes — this is the last task in Track
P.1, so this is the whole-branch check.

- [ ] **Step 7: Commit**

```bash
git add provider/aws/services/ec2/fetcher.go
git commit -m "feat(ec2): show real progress messages instead of a static spinner"
```

---

## After this plan

Run a few commands interactively (a real terminal, not piped) and watch
the spinner text change instead of sitting on "Fetching...":

```bash
./ctl rds ls
./ctl rds def <some-identifier>
./ctl ec2 ls
./ctl ec2 def <some-instance-id>
```

Confirm: (1) the message changes at least once during each command, (2)
`ctl ec2 def`/`ctl rds def` show exactly one spinner at a time, not two
overlapping ones, during the AI narration step.

Track P.2 (the remaining ~10 service packages: S3, VPC, EKS, DynamoDB,
Lambda, changes, and the RDS/EC2 files not touched here — `stats.go`,
`events.go`, `cost.go`) is the next session in Track P, applying the exact
same pattern established here. Track 0 (confirm-before-fallback) can also
now use `viewer.SetProgress` in `cost.go` following this same pattern.
