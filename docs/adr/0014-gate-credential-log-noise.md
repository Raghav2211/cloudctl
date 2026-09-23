# ADR-014: Gate credential-resolution log noise behind --debug

## Status
Accepted

## Context
`provider/aws/awsv2.go` had 7 `log.Println`/`log.Printf` calls in `NewSessionV2`/
`loadConfigFromKeys` that fired on every single command invocation regardless of any flag —
"load from direct credential", "no credential provided, fallback on profiles", "load profile
from enviornment X", etc. This has been visible noise in every live test run throughout this
whole project. A `debug bool` was already threaded through `CredentialConfig`, but it only
gated the AWS SDK's own HTTP-level request/response logging, not these lines.

While fixing this, found the fix would have been silently defeated for every EC2 command: all
four EC2 executor constructors (`NewinstanceListCommandExecutor`,
`NewInstanceDescribeCommandExecutor`, `NewEC2StatisticsDescribeCommandExecutor`,
`NewSecurityGroupExplainCommandExecutor`) hardcoded `aws.NewCredentialConfig(*flag, true)` —
always `true`, never actually reading the user's `--debug` flag. S3's commands already
correctly passed `cli.Debug` (the real flag value); EC2's never did.

## Decision
Added a small `debugLog(debug bool, format string, args ...any)` helper and replaced all 7
unconditional `log.Println`/`log.Printf` calls with it. Threaded a real `debug bool` parameter
through all four EC2 executor constructors (mirroring the same pattern already used for
`tzIdentifier` when the timezone bug was fixed, ADR-012), and updated their CLI callers to
pass `cli.Debug` — the actual flag value — instead of a hardcoded `true`. This required adding
`cli *global.CLIFlag` as a `Run()` parameter to `sgExplainCmd` (`ec2 explain`), which didn't
have it before, matching the pattern every other EC2/S3 command already uses.

## Consequences
- Every command is quiet by default now; `--debug` reveals credential-resolution internals
  when actually needed for troubleshooting.
- EC2 commands respect `--debug` for the first time — previously `NewSessionV2`'s AWS
  SDK-level HTTP logging was also always on for EC2 (since `debug` was hardcoded `true`),
  meaning EC2 commands were unconditionally noisier than S3 commands in two independent ways
  this one fix resolves together.
