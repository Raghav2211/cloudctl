package ec2

import (
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	instance_state_name_key = "instance-state-name"
)

type instanceListFilter struct {
	instanceTypes []string
}

func (f *instanceListFilter) getInstanceTypeFilter() *ec2.Filter {
	key := instance_state_name_key
	instanceTypeFilterValues := []*string{}
	for i := range f.instanceTypes {
		if f.instanceTypes[i] == "all" {
			return &ec2.Filter{}
		}
		instanceTypeFilterValues = append(instanceTypeFilterValues, &f.instanceTypes[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: instanceTypeFilterValues,
	}
	return filter
}
