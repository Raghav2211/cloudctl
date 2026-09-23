package eks

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
)

func TestClusterDefinitionEvidence_FullyPopulated(t *testing.T) {
	def := newClusterDefinition(testCluster("prod"))
	def.SetNodeGroups([]*nodeGroupSummary{
		newNodeGroupSummary(types.Nodegroup{
			NodegroupName: aws.String("default"),
			Status:        types.NodegroupStatusActive,
			CapacityType:  types.CapacityTypesOnDemand,
			InstanceTypes: []string{"m5.large"},
			ScalingConfig: &types.NodegroupScalingConfig{DesiredSize: aws.Int32(3), MinSize: aws.Int32(1), MaxSize: aws.Int32(5)},
		}),
	})
	facts := clusterDefinitionEvidence(def)

	// Version, Status, EndpointAccess, ClusterLogging + 1 NodeGroup fact
	if len(facts) != 5 {
		t.Fatalf("expected 5 facts, got %d: %+v", len(facts), facts)
	}
	for _, f := range facts {
		if f.Confidence != "FACT" {
			t.Errorf("expected all evidence to be Fact-tagged, got %q for field %q", f.Confidence, f.Field)
		}
	}
}

func TestClusterDefinitionEvidence_NilDefinition(t *testing.T) {
	if facts := clusterDefinitionEvidence(nil); facts != nil {
		t.Fatalf("expected nil facts for a nil definition, got %+v", facts)
	}
}
