# ADR-005: Adopt generic Fetcher[T].Fetch(ctx) (T, error) over interface{} + embedded errors

## Status
Accepted

## Context
`Fetch() (data interface{})` has no context and no error return; every viewer function
type-asserts the concrete output and manually checks an embedded error field. This means
`context.Context` has nowhere to travel — only two ad hoc `context.WithTimeout(context.
Background(), ...)` calls exist anywhere (`s3/client.go`), both starting fresh instead of
deriving from a caller context. Nothing is cancellable.

## Decision
Use Go generics (go.mod already declares go 1.22) to type the pipeline:
`Fetcher[T].Fetch(ctx) (T, error)`, `CommandExecutor[T]{Fetcher, Viewer}`, `Viewer` functions
taking `(T, error)`. Keep the three-stage Fetch→transform→View shape as-is; only the
signatures change. Fold the now-generic `Fetcher[T]` interface into the `executor` package
(matching where it's actually consumed) and delete the old single-method `fetcher` package.
Thread a real `context.Context` from `main.go` (bound to `os.Interrupt` via
`signal.NotifyContext`) through kong's `Run()` bindings, down through every CLI command's
`Run(ctx, ...)` method, into `Execute(ctx)`, into every `Fetch(ctx)`.

Errors that represent partial, non-fatal outcomes still valid alongside data (e.g. S3's
"bucket has more objects than the requested max-keys" notice) are kept as a named field
on the specific output struct (`notice`), distinct from the fatal `error` return — they are
not the anti-pattern this ADR removes, since they coexist with genuinely successful data
rather than substituting for a proper error return.

`*aws.ErrorInfo` now implements the `error` interface (`Error() string`), so it can be
returned directly as a `Fetch` error while still carrying its `ErrorType` severity; a new
`ctlaws.ErrorView(err) *viewer.ErrorViewer` helper extracts that severity via `errors.As`
so every viewer function's error-handling collapses to one line.

## Consequences
- Restores idiomatic Go error handling, enables `context.Context` propagation (cancellation;
  Ctrl-C during a long-running command now exits promptly instead of running to completion).
- While rewriting `s3/client.go`'s `fetchBucketObjects` for `ctx` threading, found and fixed
  a real correctness bug: the recursive pagination helper passed its accumulator slice by
  value and reassigned it locally, so the caller's `objects` slice was never actually
  updated — `ctl s3 list-objects` always returned "no object found" regardless of bucket
  contents. Fixed by passing a `*[]types.Object` and writing through the pointer.
- While rewriting `s3/executor.go`'s `NewBucketListCommandExecutor`, found and fixed a
  second bug: the command's real `--connect-timeout`/`--read-timeout` flag values were
  parsed by kong but never passed into the executor, so `bucketListFetcher.requestTimeout`
  was always the zero-value struct — `listBucket` built a `context.WithTimeout(ctx, 0)`,
  an already-expired context, so `ctl s3 ls` always failed with a deadline-exceeded error.
  Fixed by threading `cmd.AWSCLIFlag.RequestTimeout` through to the executor.
- Removes repeated type-assert-then-check-error boilerplate from every viewer function.
- While updating `provider/aws/cli/services/ec2.go`'s `Run()` signatures, found and fixed a
  third, pre-existing bug: `eC2ListCmd`/`instanceDefinitionCmd`/`ec2DescribeStatisticsCmd`
  never embedded `globals.AWSCLIFlag` (unlike every S3 command struct, which does), yet their
  `Run` methods declared a `globals *globals.AWSCLIFlag` parameter expecting kong to inject
  it — kong has no such binding (embedding alone doesn't register one; only the command
  struct's own pointer type gets auto-bound), so every EC2 command failed immediately with
  `couldn't find binding of type *globals.AWSCLIFlag`, and none of `--region`/`--profile`/
  `--accessKey`/etc. were even recognized as flags. Fixed by embedding `globals.AWSCLIFlag`
  into all three EC2 command structs and reading it off the receiver (`&cmd.AWSCLIFlag`),
  matching the pattern S3's commands already use, instead of relying on kong to inject it.
