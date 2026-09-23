# ADR-015: Add viewer.Panel; retire bucketDefinition.Pretty()'s raw struct dump

## Status
Accepted

## Context
`s3/model.go`'s `bucketDefinition.Pretty()` printed boxed SDK response structs directly via
`fmt.Println("policy", *o.policy)`, producing output like `policy &{Policy:0xc0001a2340
ResultMetadata:{...}}` — a Go struct dump, not a table, and the most concrete example of
"plain/too much text" in the whole CLI. The existing `"=== AI Summary ==="` sections
(`s3/model.go`, `ec2/viewer.go`) were plain, unstyled text headers too, visually inconsistent
with the styled tables everywhere else (see ADR-014's F.2 companion, which fixed *that*
inconsistency for tables specifically).

Separately, `bucketDefinition` stored its policy/version/tags/encryption/lifecycle fields as
type-erased `*interface{}`, set via `SetPolicy(data interface{})` etc. — flagged as a P2 issue
in the original review. The concrete typed values (`*s3.GetBucketPolicyOutput` etc.) were
already available at the exact call sites in `Fetch()` that called these setters; boxing them
into `interface{}` was pure loss, not a design need.

## Decision
1. Added `viewer.Panel` — a bordered, titled block for either freeform prose (`SetBody`, for
   AI summaries) or label/value pairs (`AddEntry`, for single-resource field displays), built
   on the same `go-pretty`/`fatih/color` machinery `TableViewer` already uses (no new
   dependency), reusing `TableStyle`/`applyBorderStyle`/`formatOutput` rather than inventing a
   parallel styling system. Explicit config only — no content-sniffing, same discipline as
   `TableStyle` (ADR-004).
2. Changed `bucketDefinition`'s fields to their concrete SDK types instead of `*interface{}`,
   and `SetPolicy`/`SetVersion`/`SetTags`/`SetEncryptionConfig`/`SetLifeCycle` to accept those
   types directly — a direct, low-cost fix now that Panel rendering needs real fields anyway.
3. Retired `Pretty()` and the `viewer.FuncViewer` workaround it required (from ADR-005).
   `bucketConfigurationViewer` now builds a `CompoundViewer` of two `Panel`s directly: one for
   the AI summary, one for the real bucket configuration fields.
4. Extracted the per-field value-formatting logic (`formatPolicy`, `formatVersioning`,
   `formatTags`, `formatEncryption`, `formatLifecycle` in `s3/evidence.go`) so both
   `bucketDefinitionEvidence` (Fact-tagged evidence for `Summarize`) and the new panel-building
   code in the viewer share one formatting implementation instead of two. They differ only in
   how they handle a failed fetch: evidence-building simply omits that fact; the panel shows
   the actual error, since a rendered view should be honest about what's missing rather than
   silently absent.
5. Applied the same `Panel` treatment to `ec2 explain`'s AI summary section for visual
   consistency between the two AI narrators.

## Consequences
- `ctl s3 def` no longer prints Go struct dumps — real field names and values in a styled
  panel, consistent with every table elsewhere in the CLI.
- Resolves the `*interface{}` type-erasure issue on `bucketDefinition` as a side effect of
  doing this properly, without a dedicated cleanup pass.
- `Panel` is immediately reusable for Track G's `ec2 def` AI narration and the bucket
  blast-radius risk verdict, rather than each new AI-enriched command reinventing its own
  ad hoc text formatting the way the first two narrators did.
