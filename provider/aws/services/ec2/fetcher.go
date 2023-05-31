package ec2

import (
	"cloudctl/provider/aws"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/service/ec2"
)

const notApplicable string = "N/A"

type instanceListFetcher struct {
	client *aws.Client
}

type instanceInfoFetcher struct {
	client *aws.Client
	id     *string
}

func (f instanceListFetcher) Fetch() (interface{}, error) {

	result, err := f.client.EC2.DescribeInstances(nil)
	// TODO : handle next token scenario
	//fmt.Println("nextToken  -- ", result.NextToken)

	instances := []*instance{}

	for _, reservation := range result.Reservations {
		if len(reservation.Instances) != 0 {
			for _, o := range reservation.Instances {

				privateIp := getPrivateIp(o)

				publicIp := getPublicIp(o)
				instances = append(instances, &instance{
					id:         o.InstanceId,
					typee:      o.InstanceType,
					state:      o.State.Name,
					az:         o.Placement.AvailabilityZone,
					publicIp:   &publicIp,
					privateIp:  &privateIp,
					launchTime: o.LaunchTime,
				})
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return &instanceListOutput{instances: instances}, nil
}

func (f instanceInfoFetcher) Fetch() (instanceDetailView interface{}, err error) {

	instanceDetailView, err = fetchInstanceDetail(f.id, f.client)
	return
}

func fetchInstanceDetail(instanceId *string, client *aws.Client) (*instanceDetailView, error) {
	// doneCh := make(chan struct{})
	volumeOutputChan := make(chan []*instanceVolume)
	sgOutputChan := make(chan *instanceSGSummary)
	eniOutputChan := make(chan []*instanceNetworkinterface)
	// defer close(doneCh)

	instancesChan := getInstacneDetail(instanceId, client)

	reservations := <-instancesChan
	if len(reservations) > 1 {
		return nil, errors.New("multiple reservation found how it's possible")
	}
	reservation := reservations[len(reservations)-1]
	if len(reservation.Instances) > 1 {
		return nil, errors.New("multiple instance found how it's possible")
	}
	instance := reservation.Instances[len(reservation.Instances)-1]

	iamProfileARN := "NA"
	if instance.IamInstanceProfile != nil {
		iamProfileARN = *instance.IamInstanceProfile.Arn
	}

	// fmt.Println("instance ", instance.NetworkInterfaces)

	summary := &instanceSummary{
		id:           *instance.InstanceId,
		publicIp:     getPublicIp(instance),
		privateIp:    getPrivateIp(instance),
		state:        *instance.State.Name,
		vpcId:        *instance.VpcId,
		typee:        *instance.InstanceType,
		iamroleArn:   iamProfileARN,
		subnetId:     *instance.SubnetId,
		privateIpDNS: *instance.PrivateDnsName,
		publicIpDNS:  *instance.PublicDnsName,
	}

	detail := &instanceDetail{
		platform:   "N/A", // TODO handle platforrm if nil
		amiId:      *instance.ImageId,
		monitor:    *instance.Monitoring.State,
		osdetails:  *instance.PlatformDetails,
		launchTime: *instance.LaunchTime,
	}
	if instance.BlockDeviceMappings != nil {
		go func() {
			defer close(volumeOutputChan)
			fetchInstanceVolume(volumeOutputChan, instance.BlockDeviceMappings, client)
		}()
	}
	if instance.NetworkInterfaces != nil {
		go func() {
			defer close(sgOutputChan)
			fetchSecurityGroupsDetail(sgOutputChan, instance.NetworkInterfaces, client)
		}()
		go func() {
			defer close(eniOutputChan)
			fetchNetworkSummary(eniOutputChan, instance.NetworkInterfaces)
		}()
	}
	// fmt.Println("eniOutputChan  ==> ", <-eniOutputChan)
	return &instanceDetailView{
		summary:      summary,
		detail:       detail,
		volumes:      <-volumeOutputChan,
		sgSummary:    <-sgOutputChan,
		ntwrkSummary: <-eniOutputChan,
	}, nil
}
func getInstacneDetail(instanceId *string, client *aws.Client) chan []*ec2.Reservation {
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

func fetchInstanceVolume(outputchan chan<- []*instanceVolume, volumemappings []*ec2.InstanceBlockDeviceMapping, client *aws.Client) {
	volumeIds := []*string{}
	op := []*instanceVolume{}
	for _, b := range volumemappings {
		volumeIds = append(volumeIds, b.Ebs.VolumeId)
	}
	data, err := client.EC2.DescribeVolumes(&ec2.DescribeVolumesInput{VolumeIds: volumeIds})
	if err != nil {
		fmt.Println("error occurred in getInstanceVolume", err.Error())
		outputchan <- op
	}

	for _, volume := range data.Volumes {

		attachments := []*volumeAttachment{}
		for _, attachment := range volume.Attachments {
			attachments = append(attachments, &volumeAttachment{
				id:                  *attachment.VolumeId,
				time:                *attachment.AttachTime,
				deleteOnTermination: *attachment.DeleteOnTermination,
				device:              *attachment.Device,
				state:               *attachment.State,
			})
		}
		kmsKey := "N/A"
		if volume.KmsKeyId != nil {
			kmsKey = *volume.KmsKeyId
		}
		op = append(op, &instanceVolume{
			attachments:  attachments,
			creationTime: *volume.CreateTime,
			size:         *volume.Size,
			isEncrypt:    *volume.Encrypted,
			kmsKey:       kmsKey,
			state:        *volume.State,
		})
	}
	outputchan <- op
}

func fetchSecurityGroupsDetail(outputChan chan<- *instanceSGSummary, enis []*ec2.InstanceNetworkInterface, client *aws.Client) {
	securityGroupIds := []*string{}
	for _, eni := range enis {
		for _, sg := range eni.Groups {
			securityGroupIds = append(securityGroupIds, sg.GroupId)
		}
	}
	data, err := client.EC2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{GroupIds: securityGroupIds})
	if err != nil {
		fmt.Println("error occured fetchSecurityGroupsDetail ", err)
	}
	instanceSgSummary := &instanceSGSummary{groupIds: securityGroupIds}
	inboundRules := []*sgInboundRule{}
	outboundRules := []*sgOutboundRule{}
	for _, sg := range data.SecurityGroups {

		sgIdWithName := fmt.Sprintf("%s(%s)", *sg.GroupId, *sg.GroupName)
		for _, inboundrule := range sg.IpPermissions {
			portRange := "ALL" // handle if IpProtocol is -1
			protocol := "ALL"  // handle if IpProtocol is -1
			if *inboundrule.IpProtocol != "-1" {
				portRange = fmt.Sprintf("%d", *inboundrule.ToPort)
				if *inboundrule.FromPort != *inboundrule.ToPort {
					portRange = fmt.Sprintf("%d-%d", *inboundrule.FromPort, *inboundrule.ToPort)
				}
				protocol = strings.ToUpper(*inboundrule.IpProtocol)
			}
			for _, userIdGroupPair := range inboundrule.UserIdGroupPairs {
				description := userIdGroupPair.Description
				if description == nil {
					description = sg.Description
				}
				inboundRules = append(inboundRules, &sgInboundRule{
					portRange: portRange,
					protocol:  protocol,
					source:    *userIdGroupPair.GroupId,
					sgId:      sgIdWithName,
					desc:      *description,
				})
			}

			for _, ipRange := range inboundrule.IpRanges {
				description := ipRange.Description
				if description == nil {
					description = sg.Description
				}
				inboundRules = append(inboundRules, &sgInboundRule{
					portRange: portRange,
					protocol:  protocol,
					source:    *ipRange.CidrIp,
					sgId:      sgIdWithName,
					desc:      *description,
				})
			}

			for _, ipRange := range inboundrule.Ipv6Ranges {
				description := ipRange.Description
				if description == nil {
					description = sg.Description
				}
				inboundRules = append(inboundRules, &sgInboundRule{
					portRange: portRange,
					protocol:  protocol,
					source:    *ipRange.CidrIpv6,
					sgId:      sgIdWithName,
					desc:      *description,
				})
			}

		}
		for _, outboundrule := range sg.IpPermissionsEgress {
			portRange := "ALL" // handle if IpProtocol is -1
			protocol := "ALL"  // handle if IpProtocol is -1
			if *outboundrule.IpProtocol != "-1" {
				portRange = fmt.Sprintf("%d", *outboundrule.FromPort)
				if (outboundrule.FromPort != nil || outboundrule.ToPort != nil) && (*outboundrule.FromPort != *outboundrule.ToPort) {
					portRange = fmt.Sprintf("%d-%d", *outboundrule.FromPort, *outboundrule.ToPort)
				}
				protocol = strings.ToUpper(*outboundrule.IpProtocol)
			}

			for _, ipRange := range outboundrule.IpRanges {
				description := ipRange.Description
				if description == nil {
					description = sg.Description
				}
				outboundRules = append(outboundRules, &sgOutboundRule{
					portRange: portRange,
					protocol:  protocol,
					source:    *ipRange.CidrIp,
					sgId:      sgIdWithName,
					desc:      *description,
				})
			}

			for _, ipRange := range outboundrule.Ipv6Ranges {
				description := ipRange.Description
				if ipRange.Description == nil {
					description = sg.Description
				}
				outboundRules = append(outboundRules, &sgOutboundRule{
					portRange: portRange,
					protocol:  protocol,
					source:    *ipRange.CidrIpv6,
					sgId:      sgIdWithName,
					desc:      *description,
				})
			}
		}
	}
	// udpate inbound Rules
	instanceSgSummary.inboundRules = inboundRules
	// update outbound Rules
	instanceSgSummary.outboundRules = outboundRules

	outputChan <- instanceSgSummary
}

func fetchNetworkSummary(outputChan chan<- []*instanceNetworkinterface, enis []*ec2.InstanceNetworkInterface) {
	networkinterfaces := []*instanceNetworkinterface{}
	for _, eni := range enis {

		sgIdWithNames := []*string{}
		for _, sg := range eni.Groups {
			o := fmt.Sprintf("%s(%s)", *sg.GroupId, *sg.GroupName)
			sgIdWithNames = append(sgIdWithNames, &o)
		}
		publicIpV4Address := "N/A"
		publicIpV4DNS := "N/A"
		if eni.Association != nil {
			publicIpV4Address = *eni.Association.PublicIp
			publicIpV4DNS = *eni.Association.PublicDnsName
		}
		networkInterface := &instanceNetworkinterface{
			id:                  *eni.NetworkInterfaceId,
			description:         *eni.Description,
			privateIpv4Add:      *eni.PrivateIpAddress,
			privateIpv4DNS:      *eni.PrivateDnsName,
			publicIpv4Add:       publicIpV4Address,
			publicIpv4DNS:       publicIpV4DNS,
			attachTime:          *eni.Attachment.AttachTime,
			attachStatus:        *eni.Status,
			vpcId:               *eni.VpcId,
			subnetId:            *eni.SubnetId,
			deleteOnTermination: *eni.Attachment.DeleteOnTermination,
			securityGroups:      sgIdWithNames,
		}
		networkinterfaces = append(networkinterfaces, networkInterface)
	}
	outputChan <- networkinterfaces
}

func getPrivateIp(instance *ec2.Instance) (privateIp string) {
	privateIp = notApplicable
	if instance.PrivateIpAddress != nil {
		privateIp = *instance.PrivateIpAddress
	}
	return
}

func getPublicIp(instance *ec2.Instance) (publicIp string) {
	publicIp = notApplicable
	if instance.PublicIpAddress != nil {
		publicIp = *instance.PublicIpAddress
	}
	return
}
