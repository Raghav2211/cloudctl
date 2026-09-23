package eks

import "github.com/aws/aws-sdk-go-v2/service/eks/types"

type clusterSummary struct {
	name *string
}

type clusterListOutput struct {
	clusters []*clusterSummary
}

type nodeGroupSummary struct {
	name          *string
	status        *string
	instanceTypes []string
	desiredSize   *int32
	minSize       *int32
	maxSize       *int32
	capacityType  string
}

// clusterDefinition is `ctl aws eks def <cluster-name>`'s output: the
// deterministic cluster/node-group configuration plus a Hypothesis-grade AI
// narration of it. aiSummary is additive, never a replacement for the raw
// fields below — if empty, aiSummaryUnavailable explains why (ADR-010),
// mirroring every other *Definition type in this codebase.
type clusterDefinition struct {
	name                   *string
	version                *string
	status                 *string
	endpoint               *string
	endpointPublicAccess   bool
	endpointPrivateAccess  bool
	publicAccessCidrs      []string
	enabledClusterLogTypes []string

	nodeGroups []*nodeGroupSummary

	aiSummary            string
	aiSummaryUnavailable string

	aiRecommendations            string
	aiRecommendationsUnavailable string
}

func newClusterSummary(name string) *clusterSummary {
	return &clusterSummary{name: &name}
}

func newClusterDefinition(c types.Cluster) *clusterDefinition {
	status := string(c.Status)
	def := &clusterDefinition{
		name:    c.Name,
		version: c.Version,
		status:  &status,
	}

	if c.Endpoint != nil {
		def.endpoint = c.Endpoint
	}
	if c.ResourcesVpcConfig != nil {
		def.endpointPublicAccess = c.ResourcesVpcConfig.EndpointPublicAccess
		def.endpointPrivateAccess = c.ResourcesVpcConfig.EndpointPrivateAccess
		def.publicAccessCidrs = c.ResourcesVpcConfig.PublicAccessCidrs
	}
	if c.Logging != nil {
		for _, l := range c.Logging.ClusterLogging {
			if l.Enabled != nil && *l.Enabled {
				for _, t := range l.Types {
					def.enabledClusterLogTypes = append(def.enabledClusterLogTypes, string(t))
				}
			}
		}
	}

	return def
}

func newNodeGroupSummary(ng types.Nodegroup) *nodeGroupSummary {
	summary := &nodeGroupSummary{
		name:          ng.NodegroupName,
		instanceTypes: ng.InstanceTypes,
		capacityType:  string(ng.CapacityType),
	}
	status := string(ng.Status)
	summary.status = &status
	if ng.ScalingConfig != nil {
		summary.desiredSize = ng.ScalingConfig.DesiredSize
		summary.minSize = ng.ScalingConfig.MinSize
		summary.maxSize = ng.ScalingConfig.MaxSize
	}
	return summary
}

func (def *clusterDefinition) SetNodeGroups(groups []*nodeGroupSummary) *clusterDefinition {
	def.nodeGroups = groups
	return def
}

func (def *clusterDefinition) SetAISummary(summary string) *clusterDefinition {
	def.aiSummary = summary
	return def
}

func (def *clusterDefinition) SetAISummaryUnavailable(reason string) *clusterDefinition {
	def.aiSummaryUnavailable = reason
	return def
}

func (def *clusterDefinition) SetAIRecommendations(recommendations string) *clusterDefinition {
	def.aiRecommendations = recommendations
	return def
}

func (def *clusterDefinition) SetAIRecommendationsUnavailable(reason string) *clusterDefinition {
	def.aiRecommendationsUnavailable = reason
	return def
}
