# ADR-022: Deterministic security findings, never AI-generated

## Status
Accepted

## Context
The roadmap's Phase 8 (Security Analysis) asks for structured `Finding{severity, resource,
evidence, remediation}` objects. Before this work, the only security-flavored output anywhere in
cloudctl was `ai.Client.Recommend`'s free-text prose — useful narration, but not something
downstream tooling (or a human doing triage) can rely on as a definitive "this is broken" claim,
since an LLM can miss, invent, or mis-prioritize an issue. This codebase's standing rule (ADR-009)
is that only deterministic code may assert a `Fact`; AI output is always `Hypothesis`/
`Recommendation` grade. A real security-findings feature needs its rules to be Facts, not prose.

## Decision
Add a new `security` package with `Severity` (Critical/High/Medium/Low), `Finding{Rule, Severity,
ResourceID, Description, Remediation, Evidence}`, and `Report{ResourceID, Findings}` — deliberately
with **no dependency on `cloudctl/ai`**. Every Finding is produced by a plain Go function evaluating
already-fetched, already-typed data (mirroring the `health` package's existing pattern of pure,
deterministic derivation with zero new API calls in most cases). AI may still narrate a `Report`
later (e.g. prioritization prose) exactly like every other command, but it never gets to invent or
contribute a Finding itself.

**One rule per service, chosen for being directly readable from data each service's `def` command
already fetches** — no speculative new API calls except one (see below):
- `ec2-public-instance-open-ingress` (HIGH): public IP + a security group ingress rule open to
  `0.0.0.0/0`.
- `rds-publicly-accessible` (HIGH): `PubliclyAccessible == true`.
- `s3-public-bucket-policy` (CRITICAL): see below — the one rule needing a new API call.
- `vpc-default-vpc-in-use` (LOW): the AWS-created default VPC is in use.
- `dynamodb-default-encryption-key` (MEDIUM): table relies on DynamoDB's default AWS-owned key
  rather than a customer-managed KMS key.
- `lambda-deprecated-runtime` (MEDIUM): function runtime is in a hardcoded, conservative
  deprecated-runtime list (needs periodic manual updates as AWS deprecates further runtimes).
- `eks-public-endpoint-access` (HIGH): cluster API server endpoint has public access enabled.

**S3's rule needed one new, real AWS API call**: `GetBucketPolicyStatus`, added to
`bucketConfigurationFetcher` alongside its existing policy/version/tags/encryption/lifecycle calls,
tolerant of the same "no policy" error every other field already tolerates. This was deliberately
chosen over a heuristic (pattern-matching the raw policy JSON already fetched for `Principal":"*"`)
because `GetBucketPolicyStatus.PolicyStatus.IsPublic` is AWS's own authoritative computation,
factoring in Block Public Access and the policy's full effect — a heuristic could produce a
plausible-looking but wrong Finding, which would violate the whole point of this ADR (Findings must
be trustworthy Facts, not best-effort guesses).

**Surface**: a new `ctl aws <service> security <id>` command per service, not folded into `def`.
Each wraps the existing `def` fetch (reusing its Fetcher unmodified — including that Fetcher's own
existing AI narration side effect, the same accepted tradeoff as ADR-020's investigation tools) and
evaluates it through the service's `xxxSecurityFindings` function. `security.Viewer` renders a
findings table sorted most-severe-first, or a clean "no findings" panel — never an unsorted list a
reader has to re-triage themselves.

## Consequences
- Findings round-trip through the exact same `--output json/yaml` machinery every other command
  uses, via `security.Report`/`Finding`'s exported fields and the shared `TableViewer`.
- Coverage is intentionally shallow (one rule per service) rather than deep on fewer services —
  each rule is a real, well-known misconfiguration class, but nowhere near exhaustive (e.g. no IAM
  policy analysis beyond what `s3 impact` already did, no network ACL analysis, no secrets-in-
  environment-variables check for Lambda). Expanding coverage is a natural, low-risk follow-up:
  each new rule is an isolated pure function, not a change to the surrounding mechanism.
- `dynamodb-default-encryption-key`'s literal string (`"default (DynamoDB owned key)"`) is now a
  shared package-level const between `model.go` (where it's set) and `security.go` (where it's
  checked), so the two can't silently drift apart — the only cross-file coupling this phase
  introduced beyond the existing `def`-fetch reuse.
- `lambda-deprecated-runtime`'s list is a maintenance burden this ADR accepts explicitly: it will
  go stale as AWS deprecates further runtimes, and needs a human to update it periodically. A
  follow-up could instead call `lambda:ListRuntimes` or track AWS's published deprecation dates,
  but that's meaningfully more work for a first version of this rule.
