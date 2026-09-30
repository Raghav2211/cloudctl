# Roadmap Gap Analysis — cloudctl

*Research only — no source code was changed to produce this document. Every line item below is
traceable to a specific file, ADR, or an explicit grep result confirming absence.*

## Purpose

A 16-phase, multi-version product roadmap was proposed for cloudctl: from an AWS resource
explorer, through AI explanation and a read-only investigation agent, to security/cost/reliability
intelligence, human-approved mutation, engineering-ecosystem integrations, and eventually
multi-cloud support. As a vision document it's strong — its core principles (deterministic
operations, AI for explanation not execution, evidence-backed conclusions, read-only before
mutation) already match this codebase's actual engineering discipline rather than contradicting
it. What it doesn't do is say how much of "Phase 0/1/5" is already built versus still greenfield.
This document answers that.

Status legend: ✅ Exists · 🟡 Partial · ❌ Absent

---

## Phase-by-phase status

### Phase 0 — CLI Foundation

| Requirement | Status | Evidence |
|---|---|---|
| Credential mechanisms (keys, env, profiles, SSO, assumed roles) | 🟡 Partial | Direct keys, env fallback, and named profiles with interactive selection all work (`provider/aws/awsv2.go`). SSO/assumed-role profiles work transparently via the AWS SDK's own profile resolution, but aren't explicitly tested or documented. |
| Identify active provider/profile/account/region/identity | ✅ Exists | `ctl whoami aws` — STS `GetCallerIdentity` plus region and credential source (`provider/aws/services/identity/`). |
| Context switching between configured AWS contexts | ✅ Exists | `ctl context ls/use/save/rm` — named, saved region/profile pairs (`provider/aws/cli/globals/context.go`), with an implicit "default" context still auto-persisting last-used values for anyone who never names one. |
| Diagnostics (creds valid, APIs reachable, region valid, account identifiable, permissions available) | 🟡 Partial | `ctl whoami aws` covers creds-valid/APIs-reachable/account-identifiable in one call, but there's no dedicated `doctor` command and no per-command IAM permission simulation. |
| Version / help | ✅ Exists | Via kong's built-in help. |
| Shell completion | ✅ Exists | `ctl install-completions`, via `github.com/willabides/kongplete` (`main.go`). |
| Configurable output format (table/JSON/YAML) | ✅ Exists | `--output`/`-o table\|json\|yaml` on every command, via a `Structurable` interface on the core viewer types (`viewer/structured.go`) rather than touching every service — see ADR-worthy note in `executor/output.go`. |
| Debug mode | 🟡 Partial | `--debug`/`-d` exists (ADR-0014) but only gates credential-resolution log lines, not general request/response tracing. |
| Readable, categorized errors | ✅ Exists | `ctlaws.AWSError` classifies into authentication/permission/throttling/not_found/connectivity/unknown, each with a one-line actionable hint (`provider/aws/error.go`). |

### Phase 1 — AWS Resource Discovery

| Requirement | Status | Evidence |
|---|---|---|
| Service coverage | 🟡 Partial | 6 of ~14 listed services: s3, ec2, dynamodb, eks, rds, vpc. Missing ECS, Lambda, ALB/NLB, SQS, SNS, Route53, IAM-as-first-class, Secrets Manager. |
| List / filter / inspect a resource | ✅ Exists | `ls`/`def` commands per service, with service-specific filters (e.g. ec2's state/type/az/vpc/subnet/has-public-ip/launchat). |
| Normalize resources into common concepts (compute, database, storage, ...) | ❌ Absent | Commands are AWS-service-named throughout; no normalization layer. |
| Cross-service natural-language search ("find everything related to payment") | ❌ Absent | No unified cross-service search exists. |
| Deterministic relationship discovery | 🟡 Partial | The store's schema (`snapshot/store.go`: `resources`, `relationships` tables) is general-purpose, but only one edge type is ever written — S3 bucket ↔ IAM principal access (`provider/aws/services/s3/impact_fetcher.go`, ADR-0016). None of ALB→TG, TG→ECS/EKS, EC2→SG, RDS→subnet-group, Route53→ALB, Lambda→IAM exist. No relationship-traversal/query method exists either — only `ListResources`, filtered by type. |

### Phase 2 — Resource Operations Visibility

| Requirement | Status | Evidence |
|---|---|---|
| Logs | ❌ Absent | Zero CloudWatch Logs integration anywhere (`cloudwatchlogs`, `FilterLogEvents`, `GetLogEvents` all absent), despite the `cloudwatchlogs` SDK dependency being present. |
| Metrics | 🟡 Partial, expanded | EC2, Lambda, and RDS each have a `stats` command backed by `GetMetricStatistics` (`provider/aws/services/{ec2,lambda,rds}/stats.go`), now also AI-narrated. Still no metrics for EKS, VPC, or DynamoDB. |
| Events (EKS/ECS/EC2/RDS/Lambda/ASG) | 🟡 Partial | RDS has `ctl aws rds events` (`rds:DescribeEvents`, AI-narrated) covering failovers/backups/restarts. EKS/ECS/EC2/Lambda/ASG still have no event-fetching code — though CloudTrail-backed `ctl changes aws` (Phase 3) covers their API-call history. |
| Health scoring (deterministic, with reasons) | ✅ Exists | `health/health.go`: `Status` (Healthy/Degraded/Unhealthy/Unknown) + `Assessment{Status, Reasons}`, computed per-service by deterministic rules and folded into every `def` command's output and evidence (via a package-local `healthEvidence()` helper, `Inference`-tagged) across ec2, dynamodb, eks, rds, vpc, lambda. |
| Dependency graph traversal (e.g. Route53→ALB→EKS→Pods→RDS) | 🟡 Partial | `snapshot.ListRelationshipsFrom`/`ListRelationshipsTo` exist as traversal methods, but only one edge type is ever written — S3 bucket ↔ IAM principal access (ADR-0016). No ALB→TG, TG→ECS/EKS, EC2→SG, RDS→subnet-group, Route53→ALB, or Lambda→IAM edges exist yet. |

### Phase 3 — Change Timeline

🟡 **Mostly covered.** `ctl changes aws` (`provider/aws/services/changes/`) queries CloudTrail
`LookupEvents`, optionally filtered to one resource (`--resource`), with `--since`/`--limit`
paginating past CloudTrail's 50-per-call cap. Each row includes timestamp, event name, source,
actor, referenced resources, and a synthesized short description — every field the roadmap's
Phase 3 asks for. `ctl aws rds events` adds RDS's own non-API-call lifecycle events (failovers,
backups, restarts) on top, via `rds:DescribeEvents` — a genuinely distinct data source from
CloudTrail, not a duplicate of it.

Since CloudTrail records every AWS API call, `ctl changes aws --resource <id>` already covers the
roadmap's named examples of EKS events, Lambda deployments, security group changes, and IAM
changes — no separate integration was needed for those. Two sources remain **❌ absent**: ECS
deployment events and Auto Scaling events. Both are blocked on a prerequisite this repo doesn't
have yet — there's no ECS or Auto Scaling service/command in cloudctl at all (Phase 1 territory),
so there's no resource for either event type to attach to.

### Phase 4 — Natural Language Cloud Query

🟡 **Core mechanism built, intentionally narrow coverage.** `ctl ask aws "<question>"`
(`nlquery/nlquery.go`) sends the query to `ai.Client.Classify` (a third capability alongside
Summarize/Recommend on the same Ollama backend), which must return JSON matching a fixed `Intent`
schema. That JSON is validated against a hard-coded whitelist — three supported intents (`find
<concept>`, `ec2 ls` with state/az/vpc/has-public-ip, `changes` with resource/event/since) — before
anything executes; an unrecognized command, subcommand, flag, or value is rejected with an
explicit reason, matching the roadmap's explicit requirement ("if the request cannot be safely
mapped... explain that the query is unsupported"). The model is never trusted to gate its own
output and never executes anything directly — a validated `Intent` dispatches to the exact same
executor an equivalent hand-typed command would use (`provider/aws/cli/services/ask.go`), and the
interpreted action is always printed before it runs.

Coverage is deliberately narrow rather than attempting all five of the roadmap's example queries:
"show production databases in ap-south-1" and "find unhealthy resources" aren't supported, because
tag-based filtering and an aggregate cross-service health scan don't exist as queryable operations
yet (health is currently computed per-`def`-command, not indexed); "show load balancers with
unhealthy targets" isn't supported because there's no load-balancer service in cloudctl at all
(same Phase 1 blocker as Phase 3's ECS/ASG gaps). These would each be a natural next slice, not a
sign the mechanism doesn't work.

### Phase 5 — AI Explanation

🟡 **Partial, coverage expanded — the mechanism is production-quality, not a prototype.**
`evidence.Evidence{Source, ResourceID, Field, Value, Confidence}` plus `ai.Client.Summarize`/
`Recommend` (`ai/ollama.go`) implement exactly the "facts vs. hypothesis vs. recommendation, AI
never invents" discipline this phase describes — including the rule that only non-LLM code may
tag `Fact`/`Inference`, and only calling code (never the model itself) may tag `Hypothesis`/
`Recommendation`. Beyond `def`-style configuration data, AI narration now also covers operational
data surfaced by Phase 2's health/metrics/events work: `lambda stats`, `rds stats`
(`provider/aws/services/lambda/stats.go`, `provider/aws/services/rds/stats.go`), `rds events`
(`provider/aws/services/rds/events.go`), and CloudTrail `changes`
(`provider/aws/services/changes/fetcher.go`) each build Fact-tagged evidence from their
already-fetched data and render an AI summary + recommendations panel above the deterministic
table/panel output, with the same capped-evidence-window and graceful-degradation behavior as the
`def` commands. Every `def` command also now folds each resource's deterministic health assessment
(`health.Assessment`) into its evidence as an `Inference`-tagged fact, so AI summaries can reason
about health without inventing it. The remaining gap this phase described — synthesizing evidence
across multiple tool calls/services into one narrative — is now covered by `ctl investigate` (see
Phase 6 below), rather than by expanding `Summarize`/`Recommend` themselves.

### Phase 6 — Read-Only Investigation Agent

✅ **Built.** `ctl investigate aws "<question>"` (`investigate/agent.go`, `docs/adr/0020-bounded-
investigation-agent.md`) is a bounded ReAct loop: each turn, a new `ai.Client.Act` call (`ai/
ollama.go`) sees the question, a whitelisted tool registry, and the history of steps so far, and
must respond with exactly one action — call a specific tool, or conclude. Every action is
validated against `investigate.Registry` before anything runs (unknown tool, missing required arg,
or unparseable JSON is rejected and logged, never executed) — the same whitelist discipline as
`nlquery.Classify`, extended from a single-shot classification to a multi-step loop. The loop is
capped at `MaxIterations = 6` tool calls and 3 consecutive invalid responses, a hard backstop
independent of what the model decides. The registry (`provider/aws/cli/services/investigate.go`)
covers all 8 existing AWS services (list + def at minimum, plus stats/events/impact/changes where
those commands exist) — ~19 tools total, each a thin wrapper reusing an existing, already-tested
Fetcher/evidence-builder/Viewer. This was a deliberate, scoped extension of ADR-0009's narrow `ai.
Client`, not a reversal of it: `Act` is one more narrow method alongside `Summarize`/`Recommend`/
`Classify`, still backed by the same single Ollama client.

### Phase 7 — Evidence-Based Investigation

✅ **Built.** `investigate.Investigation{Question, Steps, Evidence, Conclusion, Summary,
Recommendations}` is exactly the aggregating object this phase asked for: it rolls every tool
call's evidence into one timeline, plus a final AI synthesis over the full aggregated evidence
trail (reusing `ai.Client.Summarize`/`Recommend` unmodified). Every step — including
rejected/failed ones — is recorded and shown to the user, never a silent black box. Investigations
are now also a durable audit trail, not a one-shot terminal report (`snapshot/investigations.go`,
ADR-0021): every `ctl investigate aws run` persists to the same local SQLite store `ctl discover`
uses, and `ctl investigate aws list`/`show <id>` list and replay past investigations — a pure
local read, no Ollama or AWS calls — without re-running anything. Saving is best-effort: a local
store failure never hides the investigation's own result.

### Phase 8 — Security Analysis

🟡 **Partial — the mechanism is built and deliberately deterministic, coverage is shallow by
design.** `security.Finding{Rule, Severity, ResourceID, Description, Remediation, Evidence}`
(`security/`, ADR-0022) is exactly the structured-findings type this phase asks for — and, unlike
`Recommend`'s free-text prose, every Finding is produced by plain Go rules evaluating already-
fetched data, with zero dependency on `cloudctl/ai`. `ctl aws <service> security <id>` runs one
rule per service (EC2 public+open-ingress, RDS publicly-accessible, S3 public bucket policy via
AWS's own `GetBucketPolicyStatus`, VPC default-VPC-in-use, DynamoDB default-encryption-key, Lambda
deprecated-runtime, EKS public-endpoint-access) against all 7 services with a `def` command. The
gap is breadth, not architecture: no IAM policy analysis beyond `s3 impact`'s existing cross-
reference, no network ACL analysis, no secrets-in-environment-variables scanning — each would be an
isolated new rule function, not a change to the mechanism itself.

### Phase 9 — Cost Analysis

🟡 **Partial — real spend when permitted, deterministic idle signals otherwise.** `ctl cost aws`
(`cost/`, `provider/aws/services/costexplorer/`, ADR-0023) tries AWS Cost Explorer
(`ce:GetCostAndUsage`) first, grouped by service over the last `--days` (default 30) — the same
data AWS bills from. If that specific permission isn't available, it falls back to a deterministic
idle-resource scan across EC2, RDS, and Lambda, reusing each service's existing `stats` fetchers
with zero new API calls for EC2 and bounded-concurrency per-resource fetches for RDS/Lambda,
mirroring Phase 8's security-findings discipline exactly (no AI involvement, `cost.Finding`
grounded in evidence). What's not built: real per-resource dollar estimates (would need pricing
data, e.g. the AWS Price List API), and idle-scan coverage is limited to the three services that
already have a `stats` command.

### Phase 10 — Reliability Analysis

❌ **Absent entirely.** No Multi-AZ/backup/health-check/single-point-of-failure detection code.

### Phase 11 — Architecture Analysis

❌ **Absent.** Blocked on Phase 1's relationship graph being far more complete first — there's not
enough relationship data yet to summarize an architecture from.

### Phase 12 — Controlled Agentic Operations

❌ **Absent — correctly.** No plan/approve/dry-run/execute/verify/rollback framework exists, and
per this codebase's current read-only nature and the roadmap's own stated principle (mutation only
after the read-only product matures), it shouldn't yet.

### Phase 13 — Auditability

❌ **Absent.** No audit-trail or action-log feature exists in cloudctl itself; the only "audit"
hits in the repo are an unrelated test-fixture bucket name and EKS's own `LogTypeAudit` SDK
constant.

### Phase 14 — Engineering Integrations (Kubernetes, Datadog, GitHub, Jira)

❌ **Absent entirely.** The codebase imports no non-AWS provider or tool SDK.

### Phase 15 — Cross-System Incident Investigation

❌ **Absent** — depends entirely on Phase 14.

### Phase 16 — Multi-Cloud Readiness

🟡 **Mixed, improved.** `executor.Fetcher[T]`, `viewer.Viewer[T]`, and `CommandExecutor[T]` are
genuinely provider-agnostic by design already — a second cloud could reuse this core without
changes. AWS-specific coupling is still baked into `provider/aws/awsv2.go` (session/credential/
error handling) and the top-level `CLI.AWS` field in `main.go` — no `provider.Session`/
`provider.ErrorMapper` abstraction exists yet. But the "generic vocabulary" gap this section
originally flagged is now partially closed: the `concept` package (`concept/concept.go`) maps
AWS-native resource types to generic concepts (`storage`/`compute`/`database`/...), and
`ctl find <concept>` searches across services by that generic vocabulary — without renaming any
existing AWS-native command (`s3`/`ec2`/`dynamodb` stay as-is). A second provider would extend
`concept`'s mapping table rather than requiring a command-vocabulary rename.

---

## Already ahead of schedule

Several things the roadmap treats as future-phase material are already built, tested, and in
production use in this CLI today:

- **Evidence/Confidence model** (ADR-0009) — Phase 7 material, already built and enforced.
- **Additive, gracefully-degrading AI narration, now with recommendations too** (ADR-0010) —
  `ai.Client.Summarize`/`Recommend` run concurrently under one spinner; every `def` command tags
  output by epistemic status (Hypothesis vs. Recommendation), exactly Phase 5's stated discipline.
- **Deterministic health scoring** (`health/health.go`) — Phase 2 material, covering 6 of 7
  services (S3 has no operational-state concept to score).
- **Bounded concurrency as a standing convention** (ADR-0018) — used in every multi-call fetcher
  (errgroup + semaphore), relevant to the roadmap's performance non-functional requirements.
- **Correct SDK pagination** everywhere a paginator exists — same non-functional requirement.
- **A real test culture from early on** (ADR-0007 introduced the first tests) — every service
  package now has fetcher and evidence tests; this is exactly what the roadmap's "Definition of
  Done for Every Phase" asks for going forward.

## Where the release plan actually sits

Mapped against the doc's own v0.x release plan, cloudctl now sits roughly **at v0.7/early v0.9**,
with pieces of v0.8 already built: v0.1's foundation is essentially complete (diagnostics via
whoami, output format, shell completion, identity/context commands all shipped — only full
permission-simulation diagnostics and SSO/assumed-role documentation remain open), v0.2's service
coverage is 7 of ~14 services plus a working cross-service concept search, v0.3's relationship
discovery has a real query surface (not just single-edge writes) with two edge kinds, v0.4's
observability (logs/metrics/events/health) covers Lambda and RDS plus deterministic health across 6
services, v0.5's change timeline is mostly built (CloudTrail + RDS events; ECS/ASG sources blocked
on those services not existing yet), v0.6's read-only investigation agent is built end-to-end
(`ctl investigate`, ADR-0020) — a bounded, whitelisted ReAct loop over all 8 existing services,
whose evidence-aggregation byproduct also satisfies v0.8's evidence-based-investigation ask — v0.7's
structured findings mechanism now exists too (`security.Finding`/`cost.Finding`, ADR-0022/ADR-0023),
with security coverage started (one rule per service) and reliability findings not yet begun, and
v0.9's cost visibility is built for the Cost Explorer path (real spend by service) with a
deterministic idle-resource fallback when that permission isn't available. Security/cost rule
coverage is shallow by design (one rule per service, not exhaustive) — a natural next slice, not a
sign the mechanism doesn't work.

## What this document is not

This is a status map, not a recommendation. Deciding what to build next — and scoping it into an
executable plan — is a separate, later planning session. (This document itself is updated
incrementally as phases complete, rather than re-written from scratch each time — check the
phase-by-phase table above for current status, not just this closing summary.)
