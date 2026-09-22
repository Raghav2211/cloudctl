# ADR-001: Finish the EC2 SDK migration as v2, not revert to v1

## Status
Accepted

## Context
`provider/aws/services/ec2/filter.go` already uses `aws-sdk-go-v2` types. `fetcher.go` and
`model.go` still import `github.com/aws/aws-sdk-go` (v1) and reference a `ctlaws.Client`
struct that no longer exists — `provider/aws/aws.go` was deleted in favor of
`provider/aws/awsv2.go`, which only returns `*aws.Config`, not a client wrapper. The package
does not compile. Reverting `filter.go` back to v1 would also "fix" the build.

## Decision
Complete the migration forward to v2, matching S3 (already fully on v2) and the v2-style
client construction already present in `ec2/executor.go` (`ec2.NewFromConfig(*cfg)`). Fix the
`log.Fatalf` calls (ADR, folded here) and the stats data race (ADR-002) in the same pass,
since the touched functions are being rewritten anyway.

## Consequences
- One coherent SDK version across the codebase; no second migration needed later.
- Slightly larger diff than a revert would be, but a revert leaves S3/EC2 permanently
  inconsistent and someone would have to redo this migration later regardless.
- `fetchInstanceVolumeSummary`/`fetchIngressEgressRuleSummary` lose their `log.Fatalf` calls
  as part of this same rewrite (see also the dedicated note in the implementation plan,
  session 1A.3).
