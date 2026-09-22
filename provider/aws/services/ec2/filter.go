package ec2

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	instance_state_name_key = "instance-state-name"
	instance_type_key       = "instance-type"
	az_key                  = "availability-zone"
	vpc_id_key              = "vpc-id"
	subnet_id_key           = "subnet-id"
	launch_time_key         = "launch-time"
)

type InstanceListFilterOptFunc func(*InstanceListFilter)

type InstanceListFilter struct {
	instanceStates []string
	instanceTypes  []string
	azs            []string
	vpcIds         []string
	subnetIds      []string
	hasPublicIp    *bool
	launchAt       *string
}

func (f *InstanceListFilter) applyCustomFilter(instance types.Instance) bool {
	if f.hasPublicIp != nil && instance.PublicIpAddress == nil {
		return false
	}
	return true
}

// requestFilters builds the AWS API-side filter list. It must return nil,
// not an empty slice, when no filter applies — DescribeInstances rejects an
// explicitly-empty Filters list with "InvalidRequest: The request received
// was invalid.", distinct from simply omitting the field (confirmed live
// against a real account: `ctl ec2 ls` with zero flags failed outright
// until this returned nil instead of []types.Filter{}).
func (f *InstanceListFilter) requestFilters() []types.Filter {
	var filters []types.Filter
	stateFilter := f.instanceStateFilter()
	typeFilter := f.instanceTypeFilter()
	azFilter := f.azFilter()
	vpcFilter := f.vpcFilter()
	subnetFilter := f.subnetFilter()
	launchAtFilter := f.launchAtFilter()
	if stateFilter != nil {
		filters = append(filters, *stateFilter)
	}
	if typeFilter != nil {
		filters = append(filters, *typeFilter)
	}
	if azFilter != nil {
		filters = append(filters, *f.azFilter())
	}
	if vpcFilter != nil {
		filters = append(filters, *vpcFilter)
	}
	if subnetFilter != nil {
		filters = append(filters, *subnetFilter)
	}
	if launchAtFilter != nil {
		filters = append(filters, *launchAtFilter)
	}
	// log.Default().Println("requestFilters ==> ", filters)
	// log.Default().Println("customFilter ==> ", filters)
	return filters
}

func (f *InstanceListFilter) instanceTypeFilter() *types.Filter {
	if len(f.instanceTypes) == 0 {
		return nil
	}
	instanceTypeFilterValues := []string{}
	for i := range f.instanceTypes {
		instanceTypeFilterValues = append(instanceTypeFilterValues, f.instanceTypes[i])
	}
	filter := &types.Filter{
		Name:   aws.String(instance_type_key),
		Values: instanceTypeFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) instanceStateFilter() *types.Filter {
	if len(f.instanceStates) == 0 {
		return nil
	}
	instanceStateFilterValues := []string{}
	for i := range f.instanceStates {
		instanceStateFilterValues = append(instanceStateFilterValues, f.instanceStates[i])
	}
	filter := &types.Filter{
		Name:   aws.String(instance_state_name_key),
		Values: instanceStateFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) azFilter() *types.Filter {
	if len(f.azs) == 0 {
		return nil
	}

	azFilterValues := []string{}
	for i := range f.azs {
		azFilterValues = append(azFilterValues, f.azs[i])
	}
	filter := &types.Filter{
		Name:   aws.String(az_key),
		Values: azFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) vpcFilter() *types.Filter {
	if len(f.vpcIds) == 0 {
		return nil
	}
	vpcFilterValues := []string{}
	for i := range f.vpcIds {
		vpcFilterValues = append(vpcFilterValues, f.vpcIds[i])
	}
	filter := &types.Filter{
		Name:   aws.String(vpc_id_key),
		Values: vpcFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) subnetFilter() *types.Filter {
	if len(f.subnetIds) == 0 {
		return nil
	}
	subnetFilterValues := []string{}
	for i := range f.subnetIds {
		subnetFilterValues = append(subnetFilterValues, f.subnetIds[i])
	}
	filter := &types.Filter{
		Name:   aws.String(subnet_id_key),
		Values: subnetFilterValues,
	}
	return filter
}

func (f *InstanceListFilter) launchAtFilter() *types.Filter {
	if f.launchAt == nil {
		return nil
	}

	filter := &types.Filter{
		Name: aws.String(launch_time_key),
		Values: []string{
			*f.launchAt,
		},
	}
	return filter
}

func NewInstanceFilter(optfuncs ...InstanceListFilterOptFunc) *InstanceListFilter {
	filter := &InstanceListFilter{
		hasPublicIp: nil,
	}
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

func WithLaunchAt(time string) InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.launchAt = &time
	}
}

func WithHasPublicIp() InstanceListFilterOptFunc {
	return func(filter *InstanceListFilter) {
		filter.hasPublicIp = aws.Bool(true)
	}
}
