# ADR-013: SDK v2 usage confirmed complete; consolidate discover aws's duplicate session

## Status
Accepted

## Context
User request: verify every service uses AWS SDK v2. Audited via
`grep -rn "github.com/aws/aws-sdk-go/" --include="*.go" .` (the v1 import path, distinct from
`aws-sdk-go-v2`) across the whole repository, and checked go.mod for any v1 `aws-sdk-go`
entry. Both came back empty — zero v1 SDK usage remains anywhere. This work was actually
completed in Track 1A (ADR-001, the EC2 SDK migration); there is no migration left to do.

The one real inconsistency found during the audit: `DiscoverAWSCmd.Run` (added in Track 1D)
builds two separate AWS sessions in one command — `ec2.Discover` builds its own internally
(matching EC2's existing per-call-session convention from ADR-005), while `s3.Discover`
receives a pre-built `aws.Config` (matching S3's existing convention of building the session
once in the CLI layer). Both are correct v2 usage; `discover aws` is simply the first command
that needs both providers' data in a single invocation, so it's the first place this
convention split actually costs something (two credential resolutions instead of one).

## Decision
Change `ec2.Discover`'s signature to accept a pre-built `*ec2.Client` (mirroring how
`s3.Discover` accepts a pre-built `aws.Config`), and have `DiscoverAWSCmd.Run` build exactly
one session and construct both clients from it. This is scoped to the one call site that
actually needs both — it is not a signal to change EC2's or S3's individual command
executors, which each still build their own sessions per the conventions ADR-005/ADR-006
already established and verified working.

## Consequences
- `ctl discover aws` now does one credential resolution instead of two, and (once Track F.1
  gates the credential-resolution log lines behind `--debug`) prints half as much of that
  noise even before F.1 lands.
- No change to any other command's session-building behavior.
- Confirms, formally, that this codebase has no SDK v1 remnants to track or worry about
  regressing — if one is ever reintroduced, the same grep this ADR is based on will catch it.
