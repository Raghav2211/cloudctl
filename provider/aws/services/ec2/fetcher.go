package ec2

import (
	"cloudctl/provider/aws"
	"cloudctl/time"
	"cloudctl/viewer"
	"errors"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type instanceListFetcher struct {
	client *aws.Client
	tz     *time.Timezone
	filter InstanceListFilter
}

type instanceDefinitionFetcher struct {
	client *aws.Client
	tz     *time.Timezone
	id     *string
}

func (f instanceListFetcher) Fetch() interface{} {

	apiOutput, err := fetchInstanceList(f.client, f.filter)
	instancesByState := make(map[string][]*instanceSummary)
	if len(*apiOutput) == 0 {
		errorInfo := aws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
		return &instanceListOutput{instancesByState: instancesByState, err: errorInfo}
	}
	for _, o := range *apiOutput {
		instancesByState[*o.State.Name] = append(instancesByState[*o.State.Name], newInstanceSummary(o, f.tz))
	}
	if err != nil {
		errorInfo := aws.NewErrorInfo(aws.AWSError(err), viewer.ERROR, nil)
		return &instanceListOutput{instancesByState: instancesByState, err: errorInfo}
	}
	return &instanceListOutput{instancesByState: instancesByState, err: nil}
}

func (f instanceDefinitionFetcher) Fetch() interface{} {
	definition, err := fetchInstanceDefinition(f.id, f.tz, f.client)
	if err != nil {
		return &instanceDefinition{err: err} // TODO : handle specific error
	}
	return definition
}

func fetchInstanceDefinition(instanceId *string, tz *time.Timezone, client *aws.Client) (*instanceDefinition, error) {
	instanceDefinition := newInstanceDefinition()
	networkinterfaces := []*instanceNetworkinterface{}
	wg := new(sync.WaitGroup)
	wg.Add(2)

	instancesChan := fetchInstacneDetail(instanceId, client)

	reservations := <-instancesChan
	if len(reservations) > 1 {
		return nil, errors.New("multiple reservation found how it's possible")
	}
	reservation := reservations[len(reservations)-1]
	if len(reservation.Instances) > 1 {
		return nil, errors.New("multiple instance found how it's possible")
	}

	instance := reservation.Instances[len(reservation.Instances)-1]
	instanceDefinition.SetInstanceSummary(newInstanceSummary(instance, tz))
	instanceDefinition.SetInstanceDetail(newInstanceDetail(instance))

	if instance.BlockDeviceMappings != nil {
		go func() {
			defer wg.Done()
			instanceDefinition.SetVolumeSummary(fetchInstanceVolume(instance.BlockDeviceMappings, client))
		}()
	}
	if instance.NetworkInterfaces != nil {
		go func() {
			defer wg.Done()
			instanceDefinition.SetSecurityGroupSummary(fetchSecurityGroupsDetail(instance.NetworkInterfaces, client))
		}()
		for _, eni := range instance.NetworkInterfaces {
			networkinterfaces = append(networkinterfaces, newInstanceNetworkSummary(eni))
		}
		instanceDefinition.SetNetworkInterfaces(networkinterfaces)
	}
	wg.Wait()
	return instanceDefinition, nil
}
func fetchInstanceList(client *aws.Client, filter InstanceListFilter) (*[]*ec2.Instance, error) {

	var fetch func(filter []*ec2.Filter, nextMarker string, instances *[]*ec2.Instance, client *aws.Client) error

	fetch = func(filter []*ec2.Filter, nextMarker string, instances *[]*ec2.Instance, client *aws.Client) error {
		result, err := client.EC2.DescribeInstances(&ec2.DescribeInstancesInput{
			Filters:   filter,
			NextToken: &nextMarker,
		})
		if err != nil {
			return err
		}
		for _, reservation := range result.Reservations {
			*instances = append(*instances, reservation.Instances...)
		}
		if result.NextToken != nil {
			nextMarker = *result.NextToken
			if err = fetch(filter, nextMarker, instances, client); err != nil {
				return err
			}
		}
		return nil
	}
	nextMarker := ""
	instances := []*ec2.Instance{}
	apiFilter := filter.requestFilters()
	apiFilter = append(apiFilter, filter.instanceTypeFilter())
	err := fetch(apiFilter, nextMarker, &instances, client)
	return &instances, err
}
func fetchInstacneDetail(instanceId *string, client *aws.Client) chan []*ec2.Reservation {
	instancesChan := make(chan []*ec2.Reservation)
	go func() {
		defer close(instancesChan)
		data, err := client.EC2.DescribeInstances(&ec2.DescribeInstancesInput{InstanceIds: []*string{instanceId}})
		if err != nil {
			// TODO : handle error
			fmt.Println("error occurred in getInstacneDetail", err.Error())
		}
		instancesChan <- data.Reservations
	}()
	return instancesChan
}

func fetchInstanceVolume(volumemappings []*ec2.InstanceBlockDeviceMapping, client *aws.Client) []*instanceVolume {
	volumeIds := []*string{}
	volumes := []*instanceVolume{}
	for _, b := range volumemappings {
		volumeIds = append(volumeIds, b.Ebs.VolumeId)
	}
	data, err := client.EC2.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: volumeIds})
	if err != nil {
		fmt.Println("error occurred in getInstanceVolume", err.Error())
		return nil
	}
	for _, volume := range data.Volumes {
		volumes = append(volumes, newInstanceVolume(volume))
	}
	return volumes
}

func fetchSecurityGroupsDetail(enis []*ec2.InstanceNetworkInterface, client *aws.Client) *instanceSGSummary {
	securityGroupIds := []*string{}
	for _, eni := range enis {
		for _, sg := range eni.Groups {
			securityGroupIds = append(securityGroupIds, sg.GroupId)
		}
	}
	data, err := client.EC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{GroupIds: securityGroupIds})
	if err != nil {
		fmt.Println("error occured fetchSecurityGroupsDetail ", err)
		return nil
	}
	instanceSgSummary := &instanceSGSummary{groupIds: securityGroupIds}
	ingressRules := []*ingressRule{}
	egressRules := []*egressRule{}
	for _, sg := range data.SecurityGroups {
		ingressRules = append(ingressRules, newSecurityIngressRules(*sg.GroupId, *sg.GroupName, *sg.Description, sg.IpPermissions)...)
		egressRules = append(egressRules, newSecurityEgressRules(*sg.GroupId, *sg.GroupName, *sg.Description, sg.IpPermissionsEgress)...)
	}
	// udpate ingress Rules
	instanceSgSummary.ingressRules = ingressRules
	// update egress Rules
	instanceSgSummary.egressRules = egressRules

	return instanceSgSummary
}
