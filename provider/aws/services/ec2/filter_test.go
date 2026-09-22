package ec2

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestInstanceListFilter_ApplyCustomFilter_HasPublicIp(t *testing.T) {
	filter := NewInstanceFilter(WithHasPublicIp())

	withoutIP := types.Instance{}
	if filter.applyCustomFilter(withoutIP) {
		t.Error("expected instance without a public IP to be filtered out when WithHasPublicIp is set")
	}

	withIP := types.Instance{PublicIpAddress: aws.String("1.2.3.4")}
	if !filter.applyCustomFilter(withIP) {
		t.Error("expected instance with a public IP to pass when WithHasPublicIp is set")
	}
}

func TestInstanceListFilter_ApplyCustomFilter_NoOpByDefault(t *testing.T) {
	filter := NewInstanceFilter()
	if !filter.applyCustomFilter(types.Instance{}) {
		t.Error("expected no filtering when no options are set")
	}
}

func TestInstanceListFilter_RequestFilters(t *testing.T) {
	filter := NewInstanceFilter(
		WithInstanceStates([]string{"running", "stopped"}),
		WithInstanceType([]string{"t2.micro"}),
		WithAvailabilityZone([]string{"eu-west-1a"}),
		WithVpcIds([]string{"vpc-1"}),
		WithSubnetsIds([]string{"subnet-1"}),
		WithLaunchAt("2021-09-29T*"),
	)

	got := filter.requestFilters()
	want := map[string][]string{
		instance_state_name_key: {"running", "stopped"},
		instance_type_key:       {"t2.micro"},
		az_key:                  {"eu-west-1a"},
		vpc_id_key:              {"vpc-1"},
		subnet_id_key:           {"subnet-1"},
		launch_time_key:         {"2021-09-29T*"},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d filters, got %d: %+v", len(want), len(got), got)
	}
	for _, f := range got {
		wantValues, ok := want[*f.Name]
		if !ok {
			t.Errorf("unexpected filter name %q", *f.Name)
			continue
		}
		if len(f.Values) != len(wantValues) {
			t.Errorf("filter %q: expected values %v, got %v", *f.Name, wantValues, f.Values)
		}
	}
}

// Regression test: DescribeInstances rejects an explicitly-empty Filters
// list with "InvalidRequest: The request received was invalid." (confirmed
// live against a real AWS account) — requestFilters must return nil, not
// []types.Filter{}, when no filter applies.
func TestInstanceListFilter_RequestFilters_NilWhenNoOptions(t *testing.T) {
	filter := NewInstanceFilter()
	got := filter.requestFilters()
	if got != nil {
		t.Errorf("expected requestFilters to return nil (not an empty slice) with no options set, got %#v", got)
	}
}
