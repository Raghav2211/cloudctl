# cloudctl
cloudctl is a GO library that interacts with cloud providers and displays output in a human-readable fashion.

## Running locally

### Prerequisites
- Go 1.24+
- AWS credentials configured (e.g. via `~/.aws/credentials`, `~/.aws/config`, or `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` / `AWS_REGION` env vars) — cloudctl uses the default AWS SDK credential chain

### Build and run

```bash
# Fetch dependencies
go mod download

# Build the binary
go build -o cloudctl .

# Run it
./cloudctl --help
```

Or run directly without building:

```bash
go run . --help
```

### Example commands

```bash
# List S3 buckets
./cloudctl aws s3 ls

# List objects in a bucket
./cloudctl aws s3 list-objects --name <bucket-name>

# Show a bucket's configuration (policy, versioning, tags, encryption, lifecycle)
./cloudctl aws s3 def <bucket-name>

# Cross-reference IAM policies against a bucket and narrate the blast radius
./cloudctl aws s3 impact <bucket-name>

# List EC2 instances
./cloudctl aws ec2 ls

# Get EC2 instance statistics
./cloudctl aws ec2 stats

# Get EC2 instance definition
./cloudctl aws ec2 def <instance-id>

# Explain a security group's rules
./cloudctl aws ec2 explain <security-group-id>

# List/describe DynamoDB tables, EKS clusters, RDS instances & clusters, VPCs, and Lambda functions
./cloudctl aws dynamodb ls
./cloudctl aws dynamodb def --name <table-name>
./cloudctl aws eks ls
./cloudctl aws eks def --name <cluster-name>
./cloudctl aws rds ls
./cloudctl aws rds def --name <identifier>
./cloudctl aws vpc ls
./cloudctl aws vpc def --id <vpc-id>
./cloudctl aws lambda ls
./cloudctl aws lambda def --name <function-name>

# Discover every supported resource type (EC2, S3, DynamoDB, EKS, RDS, VPC, Lambda) into a
# local snapshot, so later commands can query --from-snapshot instead of hitting live AWS APIs
./cloudctl discover aws

# Show the currently active AWS identity, account, region, and credential source —
# the fast way to confirm credentials/connectivity are working before running anything else
./cloudctl whoami aws

# Lambda function statistics (invocations/errors/throttles/duration, last 24h) and recent logs
./cloudctl aws lambda stats <function-name>
./cloudctl aws lambda logs <function-name> [--since=1h] [--limit=50] [--filter=ERROR]

# RDS instance statistics (CPU, connections, free storage, read/write latency, last 24h)
./cloudctl aws rds stats <db-instance-identifier>

# Recent CloudTrail change timeline — "what changed" before an incident. Each row includes a
# synthesized short description (actor, action, resource, source), not just the raw event name.
./cloudctl changes aws --since=2h
./cloudctl changes aws --resource i-0123456789abcdef0 --since=24h --limit=100

# Filter to only one event name, or cut noisy events out of the results
./cloudctl changes aws --event=RunInstances --since=24h
./cloudctl changes aws --exclude-event=AssumeRole --exclude-event=ConsoleLogin --since=2h

# RDS lifecycle events (backups, failovers, restarts) — RDS's own event stream, distinct from
# CloudTrail: some of these (e.g. an automated failover) have no corresponding API call at all.
./cloudctl aws rds events <db-instance-identifier> --since=24h

# Deterministic security checks — one rule per service, never AI-generated
./cloudctl aws ec2 security <instance-id>
./cloudctl aws rds security <identifier>
./cloudctl aws s3 security <bucket-name>
./cloudctl aws vpc security <vpc-id>
./cloudctl aws dynamodb security <table-name>
./cloudctl aws lambda security <function-name>
./cloudctl aws eks security <cluster-name>

# Natural-language query — mapped onto one of a small set of supported read-only operations
./cloudctl ask aws "find database resources"
./cloudctl ask aws "show running ec2 instances in ap-south-1a"
./cloudctl ask aws "what changed in the last 2 hours"

# Read-only investigation agent — multi-step, adapts to what it finds
./cloudctl investigate aws run "why is orders-db slow"
./cloudctl investigate aws run "is the checkout-service lambda healthy"
./cloudctl investigate aws list
./cloudctl investigate aws show 3

# Cost analysis — real spend via Cost Explorer, or an idle-resource scan if that's not permitted
./cloudctl cost aws
./cloudctl cost aws --days=7
```

### Natural-language query (`ctl ask`)

`ctl ask aws "<question>"` sends your question to a local Ollama model, which returns a
JSON-only classification into one of three supported intents (`find <concept>`, `ec2 ls` with
`state`/`az`/`vpc`/`has-public-ip` filters, or `changes` with `resource`/`event`/`since`). That
result is validated against a hard-coded whitelist before anything runs — an unrecognized command,
flag, or value is rejected with an explicit reason, never guessed at or silently dropped. The
model never executes anything itself: a validated intent dispatches to the exact same executor the
equivalent hand-typed command would use, and the interpreted action is always printed first, e.g.:

```
Interpreted as: ec2 ls --state=running --az=ap-south-1a
```

Queries outside this whitelist (e.g. "find unhealthy resources", anything mentioning ECS/ALB/Auto
Scaling, tag-based filters like "production") aren't supported yet and will say so rather than
returning a best-effort guess.

### Read-only investigation agent (`ctl investigate`)

`ctl investigate aws run "<question>"` is a bounded, multi-step version of `ask`: instead of
mapping your question onto exactly one command, a local model chooses up to 6 read-only tool calls
one at a time — seeing each result before deciding the next step — across every AWS service
cloudctl supports (EC2, RDS, S3, EKS, VPC, DynamoDB, Lambda, plus the CloudTrail change timeline).
Every tool call is validated against a hard-coded whitelist before it runs, exactly like `ask`'s
whitelist — the model can never invoke anything beyond the listed tools, and a bad or unparseable
response is rejected and logged rather than executed. The full step-by-step timeline (what was
called, with what args, and what it returned) is always shown alongside the final AI-generated
summary and recommendations, so the investigation is never a silent black box:

```
./cloudctl investigate aws run "why is orders-db slow"

Saved as investigation #3 (see `ctl investigate aws show 3`)

Conclusion for "why is orders-db slow" (concluded)
  orders-db shows sustained CPU above 80% and elevated read latency over the last 24h; no recent
  failover or restart events explain it, pointing to a workload/query issue rather than an
  infrastructure event.

AI Summary (Hypothesis — verify against the evidence below)
  ...

AI Recommendations (Recommendation — a human must apply these)
  ...

Investigation steps (7 evidence facts gathered)
  1  rds_list    {}                          orders-db found (available, db.r5.large)
  2  rds_stats   {identifier: orders-db}     CPU 84%, read latency 12ms, connections 40
  3  rds_events  {identifier: orders-db}     no failover/restart events in the last 24h
```

This is slower than `ask` (each step can itself trigger the called command's own AI narration, on
top of the agent's own per-turn model calls) and, like every AI feature in cloudctl, degrades
gracefully: if Ollama is unreachable, `ctl investigate` says so and stops rather than guessing. See
`docs/adr/0020-bounded-investigation-agent.md` for the full design rationale.

Every run is saved to the same local store `ctl discover` uses (`~/.cloudctl/snapshot.db`), so
`ctl investigate aws list` shows past investigations (ID, question, how it ended, when) and
`ctl investigate aws show <id>` replays one in full — the same output as the original run — without
calling Ollama or AWS again. Saving is best-effort: if the local store can't be opened, the
investigation's own result still prints normally, with a warning on stderr rather than a failure.

### Security checks (`ctl aws <service> security`)

Every service has one `ctl aws <service> security <id>` command running a small set of
deterministic security rules against the same data `def` already fetches — **never AI-generated**.
A `Finding{Rule, Severity, ResourceID, Description, Remediation, Evidence}` is only ever produced
by plain Go code evaluating already-fetched facts, the same "only deterministic code may assert a
Fact" discipline every other feature in cloudctl follows. Findings render sorted most-severe-first,
or a clean "no findings" panel if every rule checked passed:

```
./cloudctl aws rds security orders-db

Security findings for orders-db (1)
  Severity  Rule                       Description                          Remediation
  HIGH      rds-publicly-accessible    This RDS instance/cluster is         Set PubliclyAccessible
                                        configured as publicly accessible…   to false and access…
```

Today's coverage is one rule per service, each a well-known, high-value misconfiguration class:
EC2 (public IP + wide-open security group), RDS (publicly accessible), S3 (public bucket policy,
via AWS's own `GetBucketPolicyStatus`), VPC (default VPC in use), DynamoDB (default AWS-owned
encryption key instead of a customer-managed KMS key), Lambda (deprecated runtime), and EKS (public
API server endpoint access). See `docs/adr/0022-deterministic-security-findings.md` for the full
design rationale, including why S3's check needed one new AWS API call instead of a JSON heuristic.

### Cost analysis (`ctl cost aws`)

`ctl cost aws` tries AWS Cost Explorer first — real spend by service over the last `--days` (default
30), the same data AWS bills from:

```
./cloudctl cost aws

Cost by service, 2026-08-01 to 2026-08-31 (total: 842.17 USD)
  Service              Amount   Unit
  Amazon EC2           510.22   USD
  Amazon RDS           201.90   USD
  Amazon S3             45.03   USD
  ...
```

If the active identity lacks `ce:GetCostAndUsage` permission specifically (any other error is
surfaced normally, never silently swallowed), it falls back to a deterministic idle-resource scan
across EC2, RDS, and Lambda — reusing each service's existing `stats` data, zero new AWS calls for
EC2, bounded-concurrency per-resource fetches for RDS/Lambda:

```
./cloudctl cost aws

Cost Explorer unavailable — showing idle-resource findings instead
  Cost Explorer access denied (...) — showing idle-resource findings across EC2, RDS, and Lambda
  instead.

Idle-resource findings (2 of 14 resources checked)
  Rule                       Resource       Description                    Recommendation
  ec2-idle-low-cpu           i-0abc123      Average CPU over 2 days is     Consider stopping or
                                              1.2% (Low).                    downsizing…
  rds-idle-low-utilization   orders-db-2    CPU 0.8%, connections 0.1      Consider stopping or
                                              over 24h — looks idle.        downsizing…
```

These aren't dollar estimates — cloudctl doesn't fetch pricing data — just concrete "this resource
looks wasted" signals grounded in the same evidence-based discipline as every other feature. See
`docs/adr/0023-cost-explorer-primary-idle-scan-fallback.md` for the full design rationale.

### Change timeline coverage

`ctl changes aws` is CloudTrail-backed, so it already covers every AWS API call across every
service — including EKS, Lambda, security group, and IAM changes — by filtering `--resource` to
the relevant ID. `ctl aws rds events` adds RDS's own non-API-call lifecycle events on top. Two
roadmap-listed sources are intentionally not built yet: ECS deployment events and Auto Scaling
events, both of which need their own AWS service added to cloudctl first (see
`docs/roadmap-gap-analysis.md`) — there's no ECS or Auto Scaling command in cloudctl at all today.

### Output format

Every command supports `--output`/`-o` (`table` — the default, `json`, or `yaml`), for scripting
and automation:

```bash
./cloudctl aws s3 ls --output json | jq '.rows[]'
./cloudctl aws ec2 def <instance-id> -o yaml
```

In `json`/`yaml` mode, only the encoded data goes to stdout — no spinner, no colored panels, no
"Time elapsed" footer — so piping into `jq` or another tool never sees anything but the structured
output.

### Health

Every `def` command includes a deterministic Health panel (`healthy` / `degraded` / `unhealthy` /
`unknown`, with reasons) derived from the resource's already-fetched status — never a new AWS
call, never an AI judgment. `ec2 def` additionally writes and shows security-group relationships
in a Related Resources panel, powered by the local snapshot store.

### Find resources by concept

Once a snapshot exists (`ctl discover aws`), search across every discovered service by a
normalized concept instead of an AWS-specific type — e.g. `database` matches both RDS and
DynamoDB:

```bash
./cloudctl find storage    # S3 buckets
./cloudctl find database   # RDS instances/clusters + DynamoDB tables
./cloudctl find compute    # EC2 instances
./cloudctl find function   # Lambda functions
./cloudctl find network    # VPCs
./cloudctl find container-cluster  # EKS clusters
```

### Context management

`--region`/`--profile` are remembered automatically between commands (last-used values persist to
`~/.cloudctl/contexts.json`), so you don't have to repeat them every time. For switching between
multiple environments explicitly, save and name a context:

```bash
# Save a named context (doesn't switch to it)
./cloudctl context save staging --region ap-south-1 --profile staging-profile
./cloudctl context save prod --region eu-west-1 --profile prod-profile

# List saved contexts (* marks the active one)
./cloudctl context ls

# Switch the active context
./cloudctl context use prod

# Remove a saved context
./cloudctl context rm staging
```

An explicit `--region`/`--profile` flag on any command always overrides the active context for
that one invocation, and updates the active context's saved value.

### Shell completion

```bash
# bash
eval "$(cloudctl install-completions)"

# add the same line to your shell's rc file (~/.bashrc, ~/.zshrc, etc.) to persist it
```

### AI narration (optional)

Every `def` command (`s3 def`, `dynamodb def`, `eks def`, `rds def`, `vpc def`, `lambda def`,
`ec2 def`), plus `s3 impact`, `ec2 explain`, `lambda stats`, `rds stats`, `rds events`, and
`changes` (CloudTrail), can additionally render an AI summary and a set of AI-generated
recommendations for the fetched data, generated by a local
[Ollama](https://ollama.com) server. This is entirely optional and additive: if Ollama isn't
running or isn't configured, the command still prints the full deterministic output — the summary
and recommendations panels just say why they're unavailable. To enable it, run Ollama locally and
set `OLLAMA_MODEL` (and optionally `OLLAMA_HOST` / `OLLAMA_TIMEOUT`) before running a command, e.g.:

```bash
OLLAMA_MODEL=deepseek-r1:7b ./cloudctl aws s3 def <bucket-name>
```

Every AI-generated summary and recommendation is clearly labeled with its epistemic status
(hypothesis to verify, or recommendation a human must apply) — cloudctl never treats AI output as
a fact on its own, and never invents facts beyond the evidence it was given.
