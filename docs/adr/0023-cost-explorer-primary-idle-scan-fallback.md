# ADR-023: Cost Explorer primary, deterministic idle-scan fallback

## Status
Accepted

## Context
The roadmap's Phase 9 (Cost Analysis) needs cost data cloudctl has never touched before — none of
its existing AWS calls read billing information. Two real approaches exist, and they answer
different questions:

1. **AWS Cost Explorer** (`ce:GetCostAndUsage`) — the authoritative source AWS itself bills from.
   Gives real dollar spend, but only account/service-level (or by cost-allocation tag, which most
   accounts haven't configured) — not per-resource, and requires a specific IAM permission many
   read-only roles don't grant.
2. **Per-resource idle/waste findings** — no new permissions: reuse EC2/RDS/Lambda utilization data
   (`stats` commands) already built to flag likely-idle resources as deterministic findings,
   mirroring ADR-022's security-findings pattern exactly. No dollar figures (this needs pricing
   data cloudctl doesn't fetch), but a concrete "this specific resource looks wasted" signal.

The user's explicit direction: try Cost Explorer first (it's the better answer when available),
and only fall back to the idle-resource scan when Cost Explorer specifically isn't accessible —
not as a parallel, always-on second view.

## Decision
`ctl aws cost` is one command with two possible output shapes, selected automatically:

- New `cost` package (mirrors `security`'s shape exactly, including having **no dependency on
  cloudctl/ai** — every `Finding` is deterministic): `CostSummary`/`ServiceCost` for the Cost
  Explorer path, `Finding`/`Report{Mode, Summary, Findings, Scanned, FallbackReason}` for the
  fallback path, and one `Viewer` that branches on `Mode`.
- New `provider/aws/services/costexplorer` package wraps `ce:GetCostAndUsage`, grouped by SERVICE
  dimension over the last `--days` (default 30), summed across any month-boundary-spanning result
  buckets into one number per service. `IsAccessDenied(err)` classifies the specific
  `AccessDeniedException`/`AccessDenied`/`UnauthorizedOperation` codes Cost Explorer returns when
  the caller lacks `ce:GetCostAndUsage` — the exact trigger for falling back, not just "any error".
  Any other error (throttling, network, a malformed request) propagates normally; a fallback must
  never mask a real problem as if it were a permissions gap.
- The idle-resource fallback reuses already-built machinery with **zero new AWS API calls**:
  - `ec2.InvestigateIdleInstances` reuses the exact same account-wide CloudWatch fetch
    `ctl aws ec2 stats` already makes (`statisticsFetcher`, already per-instance, already
    classifying `CPU_LOW`/`Moderate`/`High`) — genuinely free, no new fetching at all.
  - `rds.InvestigateIdleInstances` / `lambda.InvestigateIdleFunctions` list resources then run
    each one's existing `stats` fetcher (`dbStatisticsFetcher`/`functionStatisticsFetcher`) with
    bounded concurrency (`maxConcurrentIdleChecks = 5`, ADR-018), flagging low CPU+connections
    (RDS) or near-zero invocations (Lambda). A single resource's stats fetch failing doesn't fail
    the whole scan — it's just excluded, the same partial-failure tolerance as `ctl aws ec2 stats`.
  - All three services' scans run concurrently at the CLI layer (`provider/aws/cli/services/
    cost.go`); if one service's scan itself fails outright (e.g. a different missing permission),
    its findings are omitted but the other services' results still render — only if every service
    fails does the command surface an error, rather than silently claiming a clean account.

## Consequences
- Reusing existing `stats`/`def` fetchers for the fallback means those fetchers' own AI narration
  side effects fire too (same accepted tradeoff as ADR-020/ADR-022) — a fallback idle scan across
  many RDS instances or Lambda functions can be slow. Acceptable because this path only runs when
  Cost Explorer access is unavailable, not on the common/expected path.
- Idle findings are **not** cost estimates — no dollar amount is attached, since that needs pricing
  data (e.g. the AWS Price List API) this ADR deliberately doesn't add. `Report.Mode` makes this
  explicit to the viewer rather than presenting a "looks idle" signal as if it were a real cost
  number.
- The fallback's coverage is bounded by which services already have a `stats` command: EC2, RDS,
  and Lambda only. EKS/VPC/DynamoDB/S3 have no CloudWatch utilization fetcher to reuse today, so
  they're not part of the idle scan — a natural follow-up once those exist, not a gap in this
  design.
- `aggregateIdleScanResults` is a small pure function specifically because the three concurrent
  AWS-backed scans themselves aren't independently unit-testable without live credentials
  (consistent with this codebase's existing convention that `New*CommandExecutor`-style wiring
  isn't unit-tested) — extracting the partial-failure aggregation logic keeps that one real
  decision point covered by a fast, deterministic test.
