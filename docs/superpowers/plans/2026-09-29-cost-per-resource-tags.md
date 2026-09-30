# Per-Resource Cost via Cost-Allocation Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `ctl cost aws` can break Cost Explorer spend down by resource (not
just service), grouping by a cost-allocation tag the user configures.

**Architecture:** `costexplorer.SummaryFetcher` gains an optional
`resourceTagKey` — when set, it adds a second Cost Explorer `GroupBy`
dimension (`TAG:<key>`) alongside the existing `SERVICE` dimension, and
accumulates a parallel `[]cost.ResourceCost` alongside the existing
`[]cost.ServiceCost`. When unset (the default), behavior is byte-identical
to today. The CLI exposes this as a new `--resource-tag` flag, and
`cost.Viewer` renders a second table (service→resource) only when resource
data is present.

**Tech Stack:** Go, `github.com/aws/aws-sdk-go-v2/service/costexplorer`.

**Spec:** `/Users/raghav.joshi/.claude/plans/cloudctl-cost-aws-gives-quizzical-puzzle.md`
(Track A of the five-track cost visibility roadmap — this plan covers Track
A only).

## Global Constraints

- Resource-level grouping is opt-in via `--resource-tag`; an empty/unset
  value must produce output byte-identical to current behavior (no new
  API call shape, `ByResource` stays empty).
- A tagged resource with an empty tag value buckets under the literal
  `ResourceID` value `"untagged"` — never an error, never a dropped row.
- If Cost Explorer itself rejects the call (e.g. the tag isn't activated as
  a cost-allocation tag in the account's Billing console), that error
  propagates normally — this is a real misconfiguration the user must fix,
  not something to swallow silently.
- `cost` package keeps its existing no-dependency-on-`cloudctl/ai` rule —
  nothing in this plan touches AI.
- `ResourceCost.Unit` and `ServiceCost.Unit` come from the same
  `GetCostAndUsage` response and must not diverge.

## Review Focus

- **Unset `--resource-tag` (the common case today):** output must be
  identical to pre-change behavior — covered by
  `TestSummaryFetcher_Fetch_NoResourceTagKeyLeavesByResourceEmpty`.
- **Tag activated but a resource has no value for it:** must bucket as
  `"untagged"`, not panic on an empty string after the `$` split —
  covered by `TestSummaryFetcher_Fetch_UntaggedResourcesBucketAsUntagged`.
- **Same tag value shared by multiple cost line items across monthly
  buckets:** amounts must sum, not overwrite — covered by
  `TestSummaryFetcher_Fetch_GroupsByResourceTag` (two periods, same
  resource ID).
- **A group unexpectedly missing the second key** (fewer keys than
  requested dimensions): must not index out of range, and the service
  total must still be counted — covered by
  `TestSummaryFetcher_Fetch_GroupMissingResourceKeyStillCountsServiceTotal`.
- **Malformed tag-group key with no `$` separator at all:** `parseTagValue`
  must not panic and must return something usable rather than crash —
  covered by `TestParseTagValue`.

---

### Task 1: Group Cost Explorer spend by an optional resource tag

**Files:**
- Modify: `cloudctl/cost/report.go`
- Modify: `cloudctl/provider/aws/services/costexplorer/fetcher.go`
- Test: `cloudctl/provider/aws/services/costexplorer/fetcher_test.go`

**Interfaces:**
- Produces: `cost.ResourceCost{ResourceID string, Service string, Amount float64, Unit string}`; `cost.CostSummary.ByResource []cost.ResourceCost` (sorted descending by `Amount`, empty when no resource-tag grouping was requested); `costexplorer.NewSummaryFetcher(cfg aws.Config, days int32, resourceTagKey string) *SummaryFetcher` (signature change — third parameter added); `costexplorer.SummaryFetcher.Fetch(ctx) (*cost.CostSummary, error)` (unchanged signature, new behavior when `resourceTagKey != ""`).

- [ ] **Step 1: Write the failing tests**

Add to `cloudctl/provider/aws/services/costexplorer/fetcher_test.go` (keep
the existing tests in the file as-is; add these alongside them):

```go
func TestSummaryFetcher_Fetch_NoResourceTagKeyLeavesByResourceEmpty(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30} // resourceTagKey left as the zero value

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 0 {
		t.Fatalf("expected no per-resource breakdown when resourceTagKey is unset, got %+v", summary.ByResource)
	}
}

func TestSummaryFetcher_Fetch_GroupsByResourceTag(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-0abc123"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("30.00"), Unit: aws.String("USD")}}},
					{Keys: []string{"Amazon EC2", "Name$i-0def456"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("20.00"), Unit: aws.String("USD")}}},
				}},
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$i-0abc123"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("10.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 2 {
		t.Fatalf("expected 2 resources, got %+v", summary.ByResource)
	}
	if summary.ByResource[0].ResourceID != "i-0abc123" || summary.ByResource[0].Amount != 40.0 {
		t.Errorf("expected i-0abc123 summed across periods to 40.0 first, got %+v", summary.ByResource[0])
	}
	if summary.ByResource[1].ResourceID != "i-0def456" || summary.ByResource[1].Amount != 20.0 {
		t.Errorf("expected i-0def456 at 20.0 second, got %+v", summary.ByResource[1])
	}
	if summary.ByResource[0].Service != "Amazon EC2" {
		t.Errorf("expected resource cost tagged with its service, got %+v", summary.ByResource[0])
	}
	if len(summary.ByService) != 1 || summary.ByService[0].Amount != 60.0 {
		t.Fatalf("expected service total 60.0 unaffected by resource grouping, got %+v", summary.ByService)
	}
}

func TestSummaryFetcher_Fetch_UntaggedResourcesBucketAsUntagged(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2", "Name$"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("15.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 1 || summary.ByResource[0].ResourceID != "untagged" {
		t.Fatalf("expected untagged spend to bucket as \"untagged\", got %+v", summary.ByResource)
	}
}

func TestSummaryFetcher_Fetch_GroupMissingResourceKeyStillCountsServiceTotal(t *testing.T) {
	client := &fakeCostExplorerClient{
		out: &costexplorer.GetCostAndUsageOutput{
			ResultsByTime: []types.ResultByTime{
				{Groups: []types.Group{
					{Keys: []string{"Amazon EC2"}, Metrics: map[string]types.MetricValue{"UnblendedCost": {Amount: aws.String("50.00"), Unit: aws.String("USD")}}},
				}},
			},
		},
	}
	f := &SummaryFetcher{client: client, days: 30, resourceTagKey: "Name"}

	summary, err := f.Fetch(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(summary.ByResource) != 0 {
		t.Fatalf("expected no resource entries when a group is missing the resource key, got %+v", summary.ByResource)
	}
	if len(summary.ByService) != 1 || summary.ByService[0].Amount != 50.0 {
		t.Fatalf("expected service total to still be counted even without a resource key, got %+v", summary.ByService)
	}
}

func TestParseTagValue(t *testing.T) {
	cases := map[string]string{
		"Name$i-0abc123":  "i-0abc123",
		"Name$":            "untagged",
		"Name":             "Name",
		"Name$multi$part":  "multi$part",
	}
	for input, want := range cases {
		if got := parseTagValue(input); got != want {
			t.Errorf("parseTagValue(%q) = %q, want %q", input, got, want)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./provider/aws/services/costexplorer/... -run 'ResourceTag|GroupsByResourceTag|UntaggedResources|GroupMissingResourceKey|ParseTagValue' -v`
Expected: build failure — `resourceTagKey` field, `parseTagValue` function,
and `cost.ResourceCost`/`ByResource` don't exist yet.

- [ ] **Step 3: Add `ResourceCost` and `ByResource` to `cloudctl/cost/report.go`**

In `cloudctl/cost/report.go`, add after the existing `ServiceCost` type:

```go
// ResourceCost is one resource's spend within a CostSummary's period,
// attributed via a cost-allocation tag rather than AWS's own per-resource
// billing data (AWS doesn't expose that for most services). ResourceID is
// that tag's value; "untagged" means Cost Explorer had spend for the
// service but no value for the configured tag on it.
type ResourceCost struct {
	ResourceID string
	Service    string
	Amount     float64
	Unit       string
}
```

And add a field to `CostSummary`:

```go
type CostSummary struct {
	PeriodStart string
	PeriodEnd   string
	TotalAmount float64
	Unit        string
	ByService   []ServiceCost  // sorted descending by Amount
	ByResource  []ResourceCost // sorted descending by Amount; empty unless a resource tag was requested
}
```

- [ ] **Step 4: Update `cloudctl/provider/aws/services/costexplorer/fetcher.go`**

Add `"strings"` to the import block.

Change the `SummaryFetcher` struct and constructor:

```go
// SummaryFetcher fetches real AWS spend by service (and, if resourceTagKey
// is set, by that cost-allocation tag) over the last Days days — the same
// data AWS itself bills from.
type SummaryFetcher struct {
	client         getCostAndUsageAPI
	days           int32
	resourceTagKey string
}

func NewSummaryFetcher(cfg aws.Config, days int32, resourceTagKey string) *SummaryFetcher {
	return &SummaryFetcher{client: costexplorer.NewFromConfig(cfg), days: days, resourceTagKey: resourceTagKey}
}
```

Replace the body of `Fetch` (keep the function signature the same):

```go
func (f *SummaryFetcher) Fetch(ctx context.Context) (*cost.CostSummary, error) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -int(f.days))
	startStr, endStr := start.Format("2006-01-02"), end.Format("2006-01-02")

	groupBy := []types.GroupDefinition{
		{Type: types.GroupDefinitionTypeDimension, Key: aws.String("SERVICE")},
	}
	if f.resourceTagKey != "" {
		groupBy = append(groupBy, types.GroupDefinition{Type: types.GroupDefinitionTypeTag, Key: aws.String(f.resourceTagKey)})
	}

	out, err := f.client.GetCostAndUsage(ctx, &costexplorer.GetCostAndUsageInput{
		TimePeriod:  &types.DateInterval{Start: aws.String(startStr), End: aws.String(endStr)},
		Granularity: types.GranularityMonthly,
		Metrics:     []string{costMetric},
		GroupBy:     groupBy,
	})
	if err != nil {
		return nil, err
	}

	// Granularity=MONTHLY can return more than one bucket if the window
	// spans a calendar-month boundary — sum across every bucket so the
	// summary is one number per service (and, if requested, per resource)
	// for the whole window, not a per-month breakdown the caller would
	// have to add up itself.
	serviceTotals := map[string]float64{}
	resourceTotals := map[[2]string]float64{} // [service, resourceID] -> amount
	unit := "USD"
	for _, period := range out.ResultsByTime {
		for _, group := range period.Groups {
			if len(group.Keys) == 0 {
				continue
			}
			service := group.Keys[0]
			metric, ok := group.Metrics[costMetric]
			if !ok || metric.Amount == nil {
				continue
			}
			amount, parseErr := strconv.ParseFloat(*metric.Amount, 64)
			if parseErr != nil {
				continue
			}
			serviceTotals[service] += amount
			if metric.Unit != nil && *metric.Unit != "" {
				unit = *metric.Unit
			}

			if f.resourceTagKey != "" && len(group.Keys) >= 2 {
				resourceID := parseTagValue(group.Keys[1])
				resourceTotals[[2]string{service, resourceID}] += amount
			}
		}
	}

	byService := make([]cost.ServiceCost, 0, len(serviceTotals))
	var total float64
	for service, amount := range serviceTotals {
		byService = append(byService, cost.ServiceCost{Service: service, Amount: amount, Unit: unit})
		total += amount
	}
	sort.Slice(byService, func(i, j int) bool { return byService[i].Amount > byService[j].Amount })

	byResource := make([]cost.ResourceCost, 0, len(resourceTotals))
	for key, amount := range resourceTotals {
		byResource = append(byResource, cost.ResourceCost{Service: key[0], ResourceID: key[1], Amount: amount, Unit: unit})
	}
	sort.Slice(byResource, func(i, j int) bool { return byResource[i].Amount > byResource[j].Amount })

	return &cost.CostSummary{
		PeriodStart: startStr,
		PeriodEnd:   endStr,
		TotalAmount: total,
		Unit:        unit,
		ByService:   byService,
		ByResource:  byResource,
	}, nil
}

// parseTagValue extracts a tag's value from Cost Explorer's tag-group key
// format ("<TagKey>$<TagValue>"), returning "untagged" when the value part
// is empty — spend Cost Explorer could attribute to the service but not to
// any specific tagged resource. A key with no "$" at all (not expected from
// a real Cost Explorer response) falls back to returning it unchanged
// rather than panicking.
func parseTagValue(rawKey string) string {
	value := rawKey
	if parts := strings.SplitN(rawKey, "$", 2); len(parts) == 2 {
		value = parts[1]
	}
	if value == "" {
		return "untagged"
	}
	return value
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./provider/aws/services/costexplorer/... -v`
Expected: PASS for every test in the package, including the four
pre-existing tests (`TestSummaryFetcher_Fetch_SumsAcrossPeriodsAndSortsDescending`,
`TestSummaryFetcher_Fetch_APIError`, `TestIsAccessDenied`) — confirming no
regression — plus the five new ones from Step 1.

- [ ] **Step 6: Commit**

```bash
git add cost/report.go provider/aws/services/costexplorer/fetcher.go provider/aws/services/costexplorer/fetcher_test.go
git commit -m "feat(cost): group Cost Explorer spend by an optional resource tag"
```

---

### Task 2: Wire `--resource-tag` through the CLI

**Files:**
- Modify: `cloudctl/provider/aws/cli/services/cost.go`

**Interfaces:**
- Consumes: `cost.ResourceCost`, `CostSummary.ByResource` (Task 1);
  `costexplorer.NewSummaryFetcher(cfg, days, resourceTagKey)` (Task 1).
- Produces: `CostCmd.ResourceTag string` (new Kong-parsed flag);
  `costFetcher{cfg, days, resourceTag}` (new field, internal type — no
  external consumers).

- [ ] **Step 1: Add the flag and thread it through**

In `cloudctl/provider/aws/cli/services/cost.go`, change `CostCmd`:

```go
type CostCmd struct {
	globals.AWSCLIFlag
	Days        int32  `default:"30" help:"How many days back to analyze, for the Cost Explorer path"`
	ResourceTag string `help:"Cost-allocation tag key to break spend down by resource (e.g. 'Name'). The tag must already be activated as a cost-allocation tag in the AWS Billing console, or this call fails. Leave empty for service-level cost only."`
}
```

Change the `Fetcher` construction inside `Run`:

```go
	exec := &executor.CommandExecutor[*cost.Report]{
		Fetcher: costFetcher{cfg: *session, days: cmd.Days, resourceTag: cmd.ResourceTag},
		Viewer:  cost.Viewer,
	}
```

Change `costFetcher` and its `Fetch` method:

```go
type costFetcher struct {
	cfg         awssdk.Config
	days        int32
	resourceTag string
}

func (f costFetcher) Fetch(ctx context.Context) (*cost.Report, error) {
	summary, err := costexplorer.NewSummaryFetcher(f.cfg, f.days, f.resourceTag).Fetch(ctx)
	if err == nil {
		return &cost.Report{Mode: cost.ModeCostExplorer, Summary: summary}, nil
	}
	if !costexplorer.IsAccessDenied(err) {
		return nil, err
	}
	// ... rest of the function is unchanged
```

(Only the first three lines of `Fetch` change — the idle-scan fallback
below is untouched.)

- [ ] **Step 2: Build and run the existing package tests**

Run: `go build ./... && go test ./provider/aws/cli/services/... -v`
Expected: build succeeds (confirms this was the only remaining call site of
the now-three-argument `NewSummaryFetcher`), and both existing tests in
`cost_test.go` (`TestAggregateIdleScanResults_SkipsFailedServicesButKeepsSucceeded`,
`TestAggregateIdleScanResults_ReturnsErrorWhenEveryServiceFails`) still
pass — this task adds no new testable logic of its own, since the grouping
behavior is already covered in Task 1.

- [ ] **Step 3: Commit**

```bash
git add provider/aws/cli/services/cost.go
git commit -m "feat(cost): add --resource-tag flag to ctl cost aws"
```

---

### Task 3: Render the per-resource breakdown

**Files:**
- Modify: `cloudctl/cost/viewer.go`
- Test: `cloudctl/cost/viewer_test.go`

**Interfaces:**
- Consumes: `CostSummary.ByResource []ResourceCost` (Task 1).
- Produces: no new exported symbols — `Viewer(report, err)`'s existing
  signature and return type (`viewer.Viewer`) are unchanged; only its
  rendered content changes when `ByResource` is non-empty.

- [ ] **Step 1: Write the failing test**

Add to `cloudctl/cost/viewer_test.go`:

```go
func TestViewer_CostExplorerMode_WithResourceBreakdown(t *testing.T) {
	report := &Report{
		Mode: ModeCostExplorer,
		Summary: &CostSummary{
			PeriodStart: "2026-01-01", PeriodEnd: "2026-01-31", TotalAmount: 123.45, Unit: "USD",
			ByService:  []ServiceCost{{Service: "Amazon EC2", Amount: 100.0, Unit: "USD"}, {Service: "Amazon S3", Amount: 23.45, Unit: "USD"}},
			ByResource: []ResourceCost{{Service: "Amazon EC2", ResourceID: "i-0abc123", Amount: 80.0, Unit: "USD"}, {Service: "Amazon EC2", ResourceID: "untagged", Amount: 20.0, Unit: "USD"}},
		},
	}
	v := Viewer(report, nil)
	json := viewer.StructuredJSON(v)
	if !strings.Contains(json, "i-0abc123") {
		t.Errorf("expected the resource breakdown in the rendered output, got %s", json)
	}
	if !strings.Contains(json, "untagged") {
		t.Errorf("expected the untagged bucket in the rendered output, got %s", json)
	}
	if !strings.Contains(json, "Amazon EC2") {
		t.Errorf("expected the service breakdown still present alongside the resource breakdown, got %s", json)
	}
	v.View() // must not panic
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cost/... -run TestViewer_CostExplorerMode_WithResourceBreakdown -v`
Expected: FAIL — the resource breakdown isn't rendered yet, so `i-0abc123`
and `untagged` are absent from the structured JSON.

- [ ] **Step 3: Update `cloudctl/cost/viewer.go`**

Add a new header variable next to the existing ones:

```go
var costTableHeader = viewer.Row{"Service", "Amount", "Unit"}
var resourceCostTableHeader = viewer.Row{"Service", "Resource", "Amount", "Unit"}
var findingsTableHeader = viewer.Row{"Rule", "Resource", "Description", "Recommendation"}
```

Replace `costSummaryViewer`:

```go
func costSummaryViewer(summary *CostSummary) viewer.Viewer {
	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Cost by service, %s to %s (total: %.2f %s)", summary.PeriodStart, summary.PeriodEnd, summary.TotalAmount, summary.Unit))
	tv.AddHeader(costTableHeader)
	for _, s := range summary.ByService {
		tv.AddRow(viewer.Row{s.Service, fmt.Sprintf("%.2f", s.Amount), s.Unit})
	}
	if len(summary.ByResource) == 0 {
		return tv
	}

	rv := viewer.NewTableViewer()
	rv.SetStyle(viewer.DefaultTableStyle())
	rv.SetTitle("Cost by resource")
	rv.AddHeader(resourceCostTableHeader)
	for _, r := range summary.ByResource {
		rv.AddRow(viewer.Row{r.Service, r.ResourceID, fmt.Sprintf("%.2f", r.Amount), r.Unit})
	}

	compound := viewer.NewCompoundViewer()
	compound.AddViewer(tv)
	compound.AddViewer(rv)
	return compound
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./cost/... -v`
Expected: PASS for all tests in the package, including the pre-existing
`TestViewer_CostExplorerMode` (which has no `ByResource` data, so it must
still render as a single table, not a compound view) — confirming no
regression — plus the new test from Step 1.

- [ ] **Step 5: Run the full test suite and build**

Run: `go build ./... && go test ./...`
Expected: everything builds and passes — this is the last task in Track A,
so this is the whole-branch check before handing off for real-AWS
verification.

- [ ] **Step 6: Commit**

```bash
git add cost/viewer.go cost/viewer_test.go
git commit -m "feat(cost): render per-resource cost breakdown when available"
```

---

## After this plan

Track A ships with `--resource-tag` unset by default (no behavior change
for existing users). Verifying it against real AWS data requires an account
with cost-allocation tags actually activated — run, in your own terminal,
against an account you control:

```bash
./ctl cost aws --days 30 --resource-tag Name
```

Confirm: (1) a per-resource table appears with sensible dollar amounts,
(2) resources without the tag show up under "untagged" rather than being
dropped, (3) `./ctl cost aws --days 30` (no `--resource-tag`) still shows
only the service-level table, unchanged from before this plan.

Track B (period-over-period diff) is the next track in the roadmap and
depends on `CostSummary.ByResource` existing, which this plan provides.
