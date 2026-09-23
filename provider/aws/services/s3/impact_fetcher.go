package s3

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	"cloudctl/snapshot"
	"cloudctl/viewer"
	"context"
	"encoding/json"
	"net/url"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentIAMPolicyChecks bounds how many policies are checked
// (GetPolicyVersion + ListEntitiesForPolicy) at once. Accounts can easily
// have hundreds of customer-managed policies — fetching them one at a time
// makes this command impractically slow — but IAM's account-wide read TPS
// limits are modest, so this stays conservative rather than maximizing
// throughput.
const maxConcurrentIAMPolicyChecks = 5

// iamPolicyCrossReferenceAPI is the minimal client capability
// bucketImpactFetcher needs, letting tests substitute a fake instead of a
// real *iam.Client.
type iamPolicyCrossReferenceAPI interface {
	ListPolicies(ctx context.Context, params *iam.ListPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListPoliciesOutput, error)
	GetPolicyVersion(ctx context.Context, params *iam.GetPolicyVersionInput, optFns ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error)
	ListEntitiesForPolicy(ctx context.Context, params *iam.ListEntitiesForPolicyInput, optFns ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error)
}

type bucketImpactFetcher struct {
	client     iamPolicyCrossReferenceAPI
	bucketName string
}

// iamPolicyDocument is the minimal shape this feature reads out of a
// policy's URL-encoded JSON document — just enough to apply
// resourceMatchesBucket, nothing more.
type iamPolicyDocument struct {
	Statement []iamStatement `json:"Statement"`
}

type iamStatement struct {
	Effect   string `json:"Effect"`
	Action   any    `json:"Action"`
	Resource any    `json:"Resource"`
}

// Fetch cross-references every customer-managed IAM policy against the
// target bucket's ARN, narrates the result via Summarize, and best-effort
// persists the resulting edges into the snapshot store's relationships
// table. Mirrors bucketConfigurationFetcher/sgExplainFetcher: AI narration
// and snapshot persistence are both additive (ADR-010) — a failure in
// either never fails Fetch itself.
func (f bucketImpactFetcher) Fetch(ctx context.Context) (*bucketImpact, error) {
	impact := newBucketImpact(f.bucketName)
	targetARN := bucketARN(f.bucketName)

	var policies []types.Policy
	paginator := iam.NewListPoliciesPaginator(f.client, &iam.ListPoliciesInput{Scope: types.PolicyScopeTypeLocal})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		}
		policies = append(policies, page.Policies...)
	}

	// Each policy's GetPolicyVersion + (for matches) ListEntitiesForPolicy is
	// an independent round trip; per-index results avoid a data race on a
	// shared append from goroutines, same pattern as ec2's statisticsFetcher.
	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrentIAMPolicyChecks)
	results := make([]*policyMatch, len(policies))
	for i, policy := range policies {
		i, policy := i, policy
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			if match, ok := f.matchPolicy(gCtx, policy, targetARN); ok {
				results[i] = &match
			}
			return nil
		})
	}
	_ = g.Wait() // per-policy failures are already absorbed inside matchPolicy, not fatal to the batch

	for _, m := range results {
		if m != nil {
			impact.matches = append(impact.matches, *m)
		}
	}

	facts := bucketImpactEvidence(impact)
	impact.applyAINarration(ctx, ai.NewClientFromEnv(), facts)
	impact.saveRelationships(ctx, targetARN)

	return impact, nil
}

// matchPolicy fetches one policy's default version, decodes its document,
// and checks every statement's Resource entries against targetARN via the
// one deterministic rule (resourceMatchesBucket). Any error along the way
// (GetPolicyVersion, decoding, ListEntitiesForPolicy) is treated as "this
// policy doesn't tell us anything" rather than failing the whole command —
// a single malformed or inaccessible policy shouldn't hide every other
// finding.
func (f bucketImpactFetcher) matchPolicy(ctx context.Context, policy types.Policy, targetARN string) (policyMatch, bool) {
	if policy.Arn == nil || policy.DefaultVersionId == nil {
		return policyMatch{}, false
	}

	versionOut, err := f.client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: policy.Arn,
		VersionId: policy.DefaultVersionId,
	})
	if err != nil || versionOut.PolicyVersion == nil || versionOut.PolicyVersion.Document == nil {
		return policyMatch{}, false
	}

	decoded, err := url.QueryUnescape(*versionOut.PolicyVersion.Document)
	if err != nil {
		return policyMatch{}, false
	}
	var doc iamPolicyDocument
	if err := json.Unmarshal([]byte(decoded), &doc); err != nil {
		return policyMatch{}, false
	}

	var effect string
	var actions []string
	matched := false
	for _, stmt := range doc.Statement {
		relevant := statementActionsForBucket(toStringSlice(stmt.Resource), toStringSlice(stmt.Action), targetARN)
		if len(relevant) == 0 {
			continue
		}
		matched = true
		effect = stmt.Effect
		actions = append(actions, relevant...)
	}
	if !matched {
		return policyMatch{}, false
	}
	actions = dedupeStrings(actions)

	entities, err := f.client.ListEntitiesForPolicy(ctx, &iam.ListEntitiesForPolicyInput{PolicyArn: policy.Arn})
	var principals []string
	if err == nil {
		for _, r := range entities.PolicyRoles {
			principals = append(principals, "role/"+derefStr(r.RoleName))
		}
		for _, u := range entities.PolicyUsers {
			principals = append(principals, "user/"+derefStr(u.UserName))
		}
		for _, g := range entities.PolicyGroups {
			principals = append(principals, "group/"+derefStr(g.GroupName))
		}
	}
	if len(principals) == 0 {
		principals = []string{"(unattached)"}
	}

	return policyMatch{
		policyName: derefStr(policy.PolicyName),
		policyArn:  derefStr(policy.Arn),
		effect:     effect,
		actions:    actions,
		principals: principals,
	}, true
}

// applyAINarration sets the AI summary and recommendations (or their
// ...Unavailable fallbacks), never returning an error (ADR-010). Summarize
// and Recommend run concurrently under a single spinner rather than
// back-to-back. Mirrors sgExplanation/instanceDefinition/bucketDefinition's
// applyAINarration in this and the ec2 package.
func (impact *bucketImpact) applyAINarration(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		impact.SetAISummaryUnavailable("no customer-managed IAM policies reference this bucket")
		impact.SetAIRecommendationsUnavailable("no customer-managed IAM policies reference this bucket")
		return
	}

	type narration struct {
		summary, summaryErr             string
		recommendations, recommendedErr string
	}
	result, _ := viewer.WithSpinner("Generating AI summary and recommendations...", func() (narration, error) {
		var n narration
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			if summary, err := client.Summarize(ctx, facts); err != nil {
				n.summaryErr = err.Error()
			} else {
				n.summary = summary
			}
		}()
		go func() {
			defer wg.Done()
			if recommendations, err := client.Recommend(ctx, facts); err != nil {
				n.recommendedErr = err.Error()
			} else {
				n.recommendations = recommendations
			}
		}()
		wg.Wait()
		return n, nil
	})

	if result.summaryErr != "" {
		impact.SetAISummaryUnavailable(result.summaryErr)
	} else {
		impact.SetAISummary(result.summary)
	}
	if result.recommendedErr != "" {
		impact.SetAIRecommendationsUnavailable(result.recommendedErr)
	} else {
		impact.SetAIRecommendations(result.recommendations)
	}
}

// saveRelationships is the relationships table's first real write path
// (ADR-016): each matched policy's principals become source_id (the
// principal's own id string, e.g. "role/my-role") --kind--> target_id (the
// bucket ARN) edges under the most recent "aws" snapshot. Persistence is
// best-effort — no snapshot existing yet (nobody has run `ctl discover aws`)
// is not a failure of this command, just a note surfaced in the output.
func (impact *bucketImpact) saveRelationships(ctx context.Context, targetARN string) {
	if len(impact.matches) == 0 {
		return
	}

	store, err := snapshot.Open(snapshot.DefaultPath())
	if err != nil {
		impact.relationshipsError = err.Error()
		return
	}
	defer store.Close()

	snapshotID, err := store.LatestSnapshotID(ctx, "aws")
	if err != nil {
		impact.relationshipsError = err.Error()
		return
	}

	for _, m := range impact.matches {
		// policyMatch's fields are unexported (this package has no reason to
		// expose them outside evidence-building/rendering), so json.Marshal
		// on it directly would silently encode as "{}" — build an exported
		// view just for this persisted record.
		evidenceJSON, _ := json.Marshal(struct {
			PolicyName string   `json:"policyName"`
			PolicyArn  string   `json:"policyArn"`
			Effect     string   `json:"effect"`
			Actions    []string `json:"actions"`
		}{m.policyName, m.policyArn, m.effect, m.actions})
		for _, principal := range m.principals {
			if err := store.SaveRelationship(ctx, snapshotID, principal, targetARN, "s3:access", evidenceJSON); err != nil {
				impact.relationshipsError = err.Error()
				return
			}
			impact.relationshipsSaved++
		}
	}
}
