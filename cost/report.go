package cost

// ServiceCost is one AWS service's spend within a CostSummary's period.
type ServiceCost struct {
	Service string
	Amount  float64
	Unit    string
}

// CostSummary is real spend from AWS Cost Explorer, grouped by service.
type CostSummary struct {
	PeriodStart string
	PeriodEnd   string
	TotalAmount float64
	Unit        string
	ByService   []ServiceCost // sorted descending by Amount
}

// Mode selects which of Report's two possible shapes is populated.
type Mode string

const (
	// ModeCostExplorer means Summary is populated: real spend data was
	// available.
	ModeCostExplorer Mode = "cost_explorer"
	// ModeIdleScan means Findings is populated: Cost Explorer access
	// wasn't available, so a deterministic per-resource idle scan ran
	// instead.
	ModeIdleScan Mode = "idle_scan"
)

// Report is `ctl aws cost`'s output — exactly one of Summary or Findings is
// populated, selected by Mode.
type Report struct {
	Mode Mode

	Summary *CostSummary

	Findings []Finding
	// Scanned is how many resources the idle scan checked (including ones
	// that produced no finding) — context for "0 findings" meaning
	// "checked and clean", not "nothing was checked".
	Scanned int
	// FallbackReason explains why the idle scan ran instead of Cost
	// Explorer (e.g. the permission error), shown to the user so the
	// fallback is never a silent surprise.
	FallbackReason string
}
