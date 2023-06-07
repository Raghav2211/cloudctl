package ec2

import (
	"log"

	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	instance_state_name_key = "instance-state-name"
	instance_type_key       = "instance-type"
	az_key                  = "availability-zone"
	vpc_id_key              = "vpc-id"
	subnet_id_key           = "subnet-id"
)

var (
	stopped_state       = "stopped"
	running_state       = "running"
	terminated_state    = "terminated"
	shutting_down_state = "shutting-down"

	instanceStates = []*string{
		&stopped_state,
		&running_state,
		&terminated_state,
		&shutting_down_state,
	}
)

type InstanceListFilterOptFunc func(*InstanceListFilter)

type InstanceListFilter struct {
	instanceStates []string
	instanceTypes  []string
	azs            []string
	vpcIds         []string
	subnetIds      []string
}

func (f *InstanceListFilter) isInstanceStatesNotEmpty() bool {
	return len(f.instanceStates) > 0
}

func (f *InstanceListFilter) isInstanceTypesNotEmpty() bool {
	return len(f.instanceTypes) > 0
}

func (f *InstanceListFilter) isAzsNotEmpty() bool {
	return len(f.azs) > 0
}

func (f *InstanceListFilter) isVpcsNotEmpty() bool {
	return len(f.vpcIds) > 0
}

func (f *InstanceListFilter) isSubnetsNotEmpty() bool {
	return len(f.subnetIds) > 0
}

func (f *InstanceListFilter) requestFilters() []*ec2.Filter {
	filters := []*ec2.Filter{}
	if f.isInstanceStatesNotEmpty() {
		filters = append(filters, f.instanceStateFilter())
	}
	if f.isInstanceTypesNotEmpty() {
		filters = append(filters, f.instanceTypeFilter())
	}
	if f.isAzsNotEmpty() {
		filters = append(filters, f.azFilter())
	}
	if f.isVpcsNotEmpty() {
		filters = append(filters, f.vpcFilter())
	}
	if f.isSubnetsNotEmpty() {
		filters = append(filters, f.subnetFilter())
	}
	log.Default().Println("RequestFilters ==> ", filters)
	return filters
}

func (f *InstanceListFilter) instanceTypeFilter() *ec2.Filter {
	if len(f.instanceTypes) == 0 {
		return &ec2.Filter{}
	}
	key := instance_type_key
	instanceTypeFilterValues := []*string{}
	for i := range f.instanceTypes {
		instanceTypeFilterValues = append(instanceTypeFilterValues, &f.instanceTypes[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: instanceTypeFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) instanceStateFilter() *ec2.Filter {
	key := instance_state_name_key
	instanceStateFilterValues := []*string{}
	for i := range f.instanceStates {
		if f.instanceStates[i] == "all" {
			return &ec2.Filter{
				Name:   &key,
				Values: instanceStates,
			}
		}
		instanceStateFilterValues = append(instanceStateFilterValues, &f.instanceStates[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: instanceStateFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) azFilter() *ec2.Filter {
	if len(f.azs) == 0 {
		return &ec2.Filter{}
	}
	key := az_key
	azFilterValues := []*string{}
	for i := range f.azs {
		azFilterValues = append(azFilterValues, &f.azs[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: azFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) vpcFilter() *ec2.Filter {
	if len(f.vpcIds) == 0 {
		return &ec2.Filter{}
	}
	key := vpc_id_key
	vpcFilterValues := []*string{}
	for i := range f.vpcIds {
		vpcFilterValues = append(vpcFilterValues, &f.vpcIds[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: vpcFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) subnetFilter() *ec2.Filter {
	if len(f.subnetIds) == 0 {
		return &ec2.Filter{}
	}
	key := subnet_id_key
	subnetFilterValues := []*string{}
	for i := range f.subnetIds {
		subnetFilterValues = append(subnetFilterValues, &f.subnetIds[i])
	}
	filter := &ec2.Filter{
		Name:   &key,
		Values: subnetFilterValues,
	}
	return filter
}

func NewInstanceFilter(optfuncs ...InstanceListFilterOptFunc) *InstanceListFilter {
	filter := &InstanceListFilter{}
	for _, optfunc := range optfuncs {
		optfunc(filter)
	}
	return filter
}

func WithAvailabilityZone(azs []string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.azs = azs
	}
}

func WithInstanceStates(states []string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.instanceStates = states
	}
}

func WithInstanceType(types []string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.instanceTypes = types
	}
}

func WithVpcIds(vpcIds []string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.vpcIds = vpcIds
	}
}

func WithSubnetsIds(subnetIds []string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.subnetIds = subnetIds
	}
}
