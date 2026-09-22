# ADR-006: Fail fast non-interactively instead of adding a --non-interactive flag

## Status
Accepted

## Context
`NewSessionV2` (`provider/aws/awsv2.go`) can block indefinitely on `survey.AskOne` (region
input, profile selection) if no region/profile resolves from flags or env vars — meaning any
CI/cron/non-TTY invocation with incomplete env hangs forever waiting on stdin. On unrecoverable
failure it also calls `log.Fatalf`, killing the whole process from library code rather than
returning an error the caller (and eventually kong's own error-reporting path) can handle.

## Decision
Detect non-interactivity automatically — `term.IsTerminal(int(os.Stdin.Fd()))` — rather than
requiring callers to remember a new `--non-interactive` flag, and fail fast with a clear
`error` in that case instead of prompting. At the same time, change `NewSessionV2`'s signature
from `*aws.Config` (with `log.Fatalf` on failure) to `(*aws.Config, error)`, so every failure
path — interactive or not — returns a normal Go error instead of exiting the process directly.
This ripples into every `NewXxxCommandExecutor` constructor that builds a session internally
(EC2's three constructors) and into every CLI `Run()` method that builds one directly (S3's
four), all of which now propagate the error through the normal `error` return kong already
prints via `FatalIfErrorf`.

## Consequences
- Existing interactive usage (a human at a terminal, the tool's original use case per the
  inferred product vision) is completely unaffected — no new flag to learn, prompts still work.
- Scripted/CI usage gets a clear, immediate failure instead of a silent hang.
- Removes the last `log.Fatalf` calls in the credential/session path — library code no longer
  calls `os.Exit` anywhere in `provider/aws`.
- Slightly more "magic" than an explicit flag, but matches how most CLI tools (e.g. `git`,
  `gh`) already handle this distinction, and avoids one more flag every command has to carry.
