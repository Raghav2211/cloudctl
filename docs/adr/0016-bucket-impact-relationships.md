# ADR-016: Bucket blast-radius via IAM — the relationships table's first write path

## Status
Accepted

## Context
`ctl s3 impact <bucket>` (Track G.2) is the first command that mixes deterministic
cross-referencing with an AI-authored verdict in the same output, and the first to write into
the snapshot store's `relationships` table (schema created in Track 1D/ADR-008, unpopulated
until now). It needed three separate decisions: how to find which IAM principals can reach a
bucket, how to keep the deterministic ARN match honestly separate from the AI's risk narrative,
and how to persist the result without turning an enrichment command into a hard dependency on
the snapshot store.

## Decision
**Cross-reference scope.** Only customer-managed (`Scope=Local`) IAM policies are checked:
`iam:ListPolicies` → for each policy, `iam:GetPolicyVersion` on its default version, decoded
and matched against the bucket's ARN (`arn:aws:s3:::<bucket>`, `.../*`, or `*`) via one fixed
rule (`resourceMatchesBucket`) → for matches, `iam:ListEntitiesForPolicy` to find attached
roles/users/groups. AWS-managed policies and inline role/user policies are out of scope for
this session — customer-managed policies are where this account's actual bucket-access grants
live, and adding two more enumeration paths (inline policies per role/user, each requiring its
own list+get calls) would meaningfully widen this session without changing the shape of the
answer for the accounts this was built against. This can be extended later without changing
the evidence/relationship shape.

**Confidence tagging.** The ARN match itself is deterministic string comparison — tagged
`evidence.Inference`, built and rendered without ever calling Summarize. Only the prose risk
narrative that `Summarize` produces from that evidence is `Hypothesis`-grade, and it's declared
as such the same way every other AI narrator in this codebase does: the panel title says
"(Hypothesis — verify against the data below)", and the deterministic matches are always
rendered directly underneath it in their own table, never folded into the AI's prose. Nothing
here introduces a new tagging mechanism — it's the existing `s3 def`/`ec2 explain`/`ec2 def`
convention applied to a case that happens to also produce `Inference`-tagged evidence instead
of only `Fact`.

**Persistence is additive, not required.** Each match's principals become
`source_id --"s3:access"--> target_id` edges (principal id string → bucket ARN) written to the
most recent `"aws"` snapshot via a new `Store.SaveRelationship` method. If no snapshot exists
yet (nobody has run `ctl discover aws`), or the write fails for any reason, `Fetch` does not
fail — `relationshipsError` is surfaced in the output instead, mirroring ADR-010's AI-is-additive
discipline extended to persistence. The command's primary value (surfacing the cross-reference
right now) never depends on snapshot infrastructure being present.

**Bounded concurrency.** Real accounts can have hundreds of customer-managed policies (this
project's own test account has 194) — `GetPolicyVersion` and `ListEntitiesForPolicy` run
concurrently (bounded to 5 in flight, `errgroup`+semaphore, same pattern as
`ec2/fetcher.go`'s CloudWatch stats fetcher) rather than sequentially, which would make the
command impractically slow on any account of meaningful size. 5 was chosen conservatively
against IAM's modest account-wide read TPS limits rather than maximizing throughput.

## Consequences
- `relationships` now has a real, tested write path instead of an unpopulated schema — future
  cross-reference features (e.g. EC2 instance profile → resource access) can reuse
  `SaveRelationship` directly.
- Confidence discipline holds even with a third tag (`Inference`) in play: nothing here lets
  the model self-report certainty, and the raw matches are always visible next to the verdict
  for a human to check.
- Scope is intentionally narrower than "every way a principal could reach this bucket" —
  inline policies, group-inherited access, and resource-based bucket policies (already shown by
  `ctl s3 def`) are not cross-referenced here. This command answers "which customer-managed
  policies grant access," not the full IAM reachability graph.
- A single unreachable/misbehaving policy (a `GetPolicyVersion` failure, a document that
  doesn't decode) is skipped, not fatal — found and verified while building this feature by
  injecting a synthetic per-policy failure in `TestBucketImpactFetcher_Fetch_DegradesGracefullyOnPerPolicyErrors`.
