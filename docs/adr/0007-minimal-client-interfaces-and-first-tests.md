# ADR-007: Extract minimal per-fetcher client interfaces, not one shared AWSClient interface

## Status
Accepted

## Context
Fetchers hold concrete `*s3.Client`/`*ec2.Client` types directly, so nothing is
unit-testable without hitting real AWS or LocalStack. There are zero test files
anywhere in the repo.

## Decision
Define narrow interfaces at each point of use, covering only the method(s) a given fetcher
actually calls, rather than one large shared `AWSClient` interface covering every SDK method
ever used anywhere. Where the AWS SDK already publishes a matching interface for this purpose
(e.g. `ec2.DescribeInstancesAPIClient`, used by `ec2.NewDescribeInstancesPaginator`), reuse it
instead of hand-rolling an equivalent — `fetchInstanceList` now takes
`ec2.DescribeInstancesAPIClient` instead of `*ec2.Client`. Where the SDK doesn't publish one
(S3's `ListBuckets`/`ListObjects`, which predate the SDK's paginator-interface convention),
hand-roll a one-method interface named for the call it wraps (`listBucketsAPI`,
`listObjectsAPI`).

Scope this pass to exactly the call paths this session adds tests for — EC2 instance-list
fetch (filter application + the paginated fetch itself) and S3 bucket-list fetch (filter
application + the object-pagination termination logic, which is also the function where a
real pagination bug was found and fixed in Track 1B). Do not sweep every remaining SDK call
site (S3's `GetBucketPolicy`/`GetBucketTagging`/etc., EC2's `DescribeVolumes`/
`DescribeSecurityGroups`/CloudWatch) in this pass — extract those interfaces when those
specific fetchers are next touched, per the existing "done per-package as touched" priority,
not as a standalone sweep.

## Consequences
- Matches the "small interfaces defined near consumers" principle already established for
  `Fetcher`/`Viewer`/`executor.Fetcher[T]`.
- `bucketListFetcher.client` changes from a concrete `s3.Client` value to the `listBucketsAPI`
  interface; `fetchInstanceList`'s `client` parameter changes from `*ec2.Client` to
  `ec2.DescribeInstancesAPIClient`; `fetchBucketObjects`'s `client` parameter changes from
  `*s3.Client` to `listObjectsAPI`. Production call sites are unaffected — `*ec2.Client` and
  `*s3.Client` already satisfy these interfaces, so `ec2.NewFromConfig`/`s3.NewFromConfig`
  results pass through unchanged.
- First tests in the repo: EC2 filter predicate/request-filter construction, EC2 paginated
  instance fetch (happy path, empty-result path, multi-page termination), S3 bucket-name/
  creation-date filter predicates, S3 object pagination (termination on `IsTruncated=false`,
  the max-keys/notice path, and a regression test for the Track 1B pagination-by-value bug).
  `go test -race ./...` now runs for the first time, exercising the Track 1A `errgroup`
  concurrency fix.
