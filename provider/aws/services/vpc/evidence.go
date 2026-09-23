package vpc

import (
	"cloudctl/evidence"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// derefStr returns "-" for a nil pointer instead of dereferencing it.
func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}

// routeTableIsPublic is the one deterministic rule this feature relies on:
// a route table is "public" iff it has a route whose GatewayId points at an
// internet gateway (igw-*) — NAT gateway routes, local routes, and peering
// routes don't make a subnet internet-reachable from the outside. This is a
// fixed rule, not a judgment call, so classifySubnets' result is
// Inference-tagged evidence, never Hypothesis.
func routeTableIsPublic(rt types.RouteTable) bool {
	for _, r := range rt.Routes {
		if r.GatewayId != nil && strings.HasPrefix(*r.GatewayId, "igw-") {
			return true
		}
	}
	return false
}

// classifySubnets builds subnetInfo for every subnet, determining
// public/private via routeTableIsPublic on each subnet's route table —
// its own explicit association if one exists, otherwise the VPC's main
// (implicit) route table, matching how AWS itself resolves routing for an
// unassociated subnet.
func classifySubnets(subnets []types.Subnet, routeTables []types.RouteTable) []*subnetInfo {
	publicByRouteTable := make(map[string]bool, len(routeTables))
	subnetToRouteTable := make(map[string]string)
	var mainRouteTableID string
	for _, rt := range routeTables {
		id := derefStr(rt.RouteTableId)
		publicByRouteTable[id] = routeTableIsPublic(rt)
		for _, assoc := range rt.Associations {
			if assoc.Main != nil && *assoc.Main {
				mainRouteTableID = id
			}
			if assoc.SubnetId != nil {
				subnetToRouteTable[*assoc.SubnetId] = id
			}
		}
	}

	infos := make([]*subnetInfo, 0, len(subnets))
	for _, s := range subnets {
		routeTableID, explicit := subnetToRouteTable[derefStr(s.SubnetId)]
		if !explicit {
			routeTableID = mainRouteTableID
		}
		infos = append(infos, &subnetInfo{
			id:               s.SubnetId,
			cidr:             s.CidrBlock,
			az:               s.AvailabilityZone,
			isPublic:         publicByRouteTable[routeTableID],
			availableIPCount: s.AvailableIpAddressCount,
		})
	}
	return infos
}

// vpcDefinitionEvidence converts an already-fetched vpcDefinition's topology
// into evidence for Summarize. The subnet public/private split is
// Inference-tagged (derived via classifySubnets' fixed rule, not an AI
// judgment call) — everything else is a direct API field (Fact).
func vpcDefinitionEvidence(def *vpcDefinition) []evidence.Evidence {
	if def == nil || def.id == nil {
		return nil
	}
	id := derefStr(def.id)

	facts := []evidence.Evidence{
		{Source: "ec2:DescribeVpcs", ResourceID: id, Field: "CIDR", Value: derefStr(def.cidr), Confidence: evidence.Fact},
		{Source: "ec2:DescribeVpcs", ResourceID: id, Field: "IsDefault", Value: def.isDefault, Confidence: evidence.Fact},
		{Source: "ec2:DescribeVpcs", ResourceID: id, Field: "State", Value: derefStr(def.state), Confidence: evidence.Fact},
	}

	publicCount, privateCount := 0, 0
	for _, s := range def.subnets {
		if s.isPublic {
			publicCount++
		} else {
			privateCount++
		}
		classification := "private"
		if s.isPublic {
			classification = "public"
		}
		facts = append(facts, evidence.Evidence{
			Source: "vpc:route-table-classification", ResourceID: derefStr(s.id), Field: "SubnetClassification",
			Value:      fmt.Sprintf("%s (%s, %s)", classification, derefStr(s.cidr), derefStr(s.az)),
			Confidence: evidence.Inference,
		})
	}
	facts = append(facts, evidence.Evidence{
		Source: "vpc:route-table-classification", ResourceID: id, Field: "SubnetSplit",
		Value: fmt.Sprintf("%d public, %d private", publicCount, privateCount), Confidence: evidence.Inference,
	})

	facts = append(facts, evidence.Evidence{
		Source: "ec2:DescribeInternetGateways", ResourceID: id, Field: "InternetGatewayCount",
		Value: len(def.internetGateways), Confidence: evidence.Fact,
	})
	facts = append(facts, evidence.Evidence{
		Source: "ec2:DescribeNatGateways", ResourceID: id, Field: "NatGatewayCount",
		Value: len(def.natGateways), Confidence: evidence.Fact,
	})

	return facts
}
