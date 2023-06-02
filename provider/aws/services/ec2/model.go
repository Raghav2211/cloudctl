package ec2

import (
	"cloudctl/provider/aws"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
)

type instance struct {
	id         *string
	typee      *string
	state      *string
	az         *string
	publicIp   *string
	privateIp  *string
	launchTime *time.Time
}

type ingressRule struct {
	portRange string
	protocol  string
	source    string
	sgId      string
	desc      string
}
type egressRule struct {
	portRange string
	protocol  string
	source    string
	sgId      string
	desc      string
}
type instanceSGSummary struct {
	groupIds     []*string
	ingressRules []*ingressRule
	egressRules  []*egressRule
}

type instanceDetail struct {
	platform   string
	amiId      string
	monitor    string
	osdetails  string
	launchTime time.Time
}
type instanceSummary struct {
	id string

	publicIp    string
	publicIpDNS string

	privateIp    string
	privateIpDNS string

	state string
	typee string

	vpcId    string
	subnetId string

	iamroleArn string
}

type volumeAttachment struct {
	id                  string
	time                time.Time
	deleteOnTermination bool
	device              string
	state               string
}

type instanceVolume struct {
	creationTime time.Time
	size         int64
	isEncrypt    bool
	kmsKey       string
	state        string
	attachments  []*volumeAttachment
}

type instanceNetworkinterface struct {
	id                  string
	description         string
	privateIpv4Add      string
	privateIpv4DNS      string
	publicIpv4Add       string
	publicIpv4DNS       string
	attachTime          time.Time
	attachStatus        string
	vpcId               string
	subnetId            string
	deleteOnTermination bool
	securityGroups      []*string
}

type instanceDefinition struct {
	summary      *instanceSummary
	detail       *instanceDetail
	volumes      []*instanceVolume
	sgSummary    *instanceSGSummary
	ntwrkSummary []*instanceNetworkinterface
	err          error
}

type instanceListOutput struct {
	instances []*instance
	err       *aws.ErrorInfo
}

func NewInstanceOutput(o *ec2.Instance) *instance {
	privateIp := getPrivateIp(o)
	publicIp := getPublicIp(o)
	return &instance{
		id:         o.InstanceId,
		typee:      o.InstanceType,
		state:      o.State.Name,
		az:         o.Placement.AvailabilityZone,
		publicIp:   &publicIp,
		privateIp:  &privateIp,
		launchTime: o.LaunchTime,
	}
}

func newInstanceSummary(instance *ec2.Instance) *instanceSummary {
	iamProfileARN := "NA"
	if instance.IamInstanceProfile != nil {
		iamProfileARN = *instance.IamInstanceProfile.Arn
	}
	return &instanceSummary{
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
}

func newInstanceDetail(instance *ec2.Instance) *instanceDetail {
	return &instanceDetail{
		platform:   "N/A", // TODO handle platforrm if nil
		amiId:      *instance.ImageId,
		monitor:    *instance.Monitoring.State,
		osdetails:  *instance.PlatformDetails,
		launchTime: *instance.LaunchTime,
	}
}

func NewInstanceVolume(volume *ec2.Volume) *instanceVolume {
	attachments := []*volumeAttachment{}
	kmsKey := "N/A"
	if volume.KmsKeyId != nil {
		kmsKey = *volume.KmsKeyId
	}
	for _, attachment := range volume.Attachments {
		attachments = append(attachments, newVolumeAttachment(attachment))
	}
	return &instanceVolume{
		attachments:  attachments,
		creationTime: *volume.CreateTime,
		size:         *volume.Size,
		isEncrypt:    *volume.Encrypted,
		kmsKey:       kmsKey,
		state:        *volume.State,
	}
}
func NewSecurityIngressRules(securityGroupId, securityGroupName, securityGroupDescription string, ingressPermissions []*ec2.IpPermission) (ingressRules []*ingressRule) {
	ingressRules = []*ingressRule{}
	sgIdWithName := fmt.Sprintf("%s(%s)", securityGroupId, securityGroupName)
	portRange := "ALL" //  if IpProtocol is -1
	protocol := "ALL"  //  if IpProtocol is -1
	for _, permission := range ingressPermissions {
		if *permission.IpProtocol != "-1" {
			portRange = fmt.Sprintf("%d", *permission.ToPort)
			if *permission.FromPort != *permission.ToPort {
				portRange = fmt.Sprintf("%d-%d", *permission.FromPort, *permission.ToPort)
			}
			protocol = strings.ToUpper(*permission.IpProtocol)
		}
		for _, userIdGroupPair := range permission.UserIdGroupPairs {
			description := userIdGroupPair.Description
			if description == nil {
				description = &securityGroupDescription
			}
			ingressRules = append(ingressRules, &ingressRule{
				portRange: portRange,
				protocol:  protocol,
				source:    *userIdGroupPair.GroupId,
				sgId:      sgIdWithName,
				desc:      *description,
			})
		}

		for _, ipRange := range permission.IpRanges {
			description := ipRange.Description
			if description == nil {
				description = &securityGroupDescription
			}
			ingressRules = append(ingressRules, &ingressRule{
				portRange: portRange,
				protocol:  protocol,
				source:    *ipRange.CidrIp,
				sgId:      sgIdWithName,
				desc:      *description,
			})
		}

		for _, ipRange := range permission.Ipv6Ranges {
			description := ipRange.Description
			if description == nil {
				description = &securityGroupDescription
			}
			ingressRules = append(ingressRules, &ingressRule{
				portRange: portRange,
				protocol:  protocol,
				source:    *ipRange.CidrIpv6,
				sgId:      sgIdWithName,
				desc:      *description,
			})
		}
	}
	return
}

func NewSecurityEgressRules(securityGroupId, securityGroupName, securityGroupDescription string, egressPermissions []*ec2.IpPermission) (egressRules []*egressRule) {
	egressRules = []*egressRule{}
	sgIdWithName := fmt.Sprintf("%s(%s)", securityGroupId, securityGroupName)
	for _, rule := range egressPermissions {
		portRange := "ALL" // handle if IpProtocol is -1
		protocol := "ALL"  // handle if IpProtocol is -1
		if *rule.IpProtocol != "-1" {
			portRange = fmt.Sprintf("%d", *rule.FromPort)
			if (rule.FromPort != nil || rule.ToPort != nil) && (*rule.FromPort != *rule.ToPort) {
				portRange = fmt.Sprintf("%d-%d", *rule.FromPort, *rule.ToPort)
			}
			protocol = strings.ToUpper(*rule.IpProtocol)
		}

		for _, ipRange := range rule.IpRanges {
			description := ipRange.Description
			if description == nil {
				description = &securityGroupDescription
			}
			egressRules = append(egressRules, &egressRule{
				portRange: portRange,
				protocol:  protocol,
				source:    *ipRange.CidrIp,
				sgId:      sgIdWithName,
				desc:      *description,
			})
		}

		for _, ipRange := range rule.Ipv6Ranges {
			description := ipRange.Description
			if ipRange.Description == nil {
				description = &securityGroupDescription
			}
			egressRules = append(egressRules, &egressRule{
				portRange: portRange,
				protocol:  protocol,
				source:    *ipRange.CidrIpv6,
				sgId:      sgIdWithName,
				desc:      *description,
			})
		}
	}
	return
}
func newVolumeAttachment(attachment *ec2.VolumeAttachment) *volumeAttachment {
	return &volumeAttachment{
		id:                  *attachment.VolumeId,
		time:                *attachment.AttachTime,
		deleteOnTermination: *attachment.DeleteOnTermination,
		device:              *attachment.Device,
		state:               *attachment.State,
	}
}
