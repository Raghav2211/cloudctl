package ec2

import (
	"cloudctl/provider/aws"
	ctltime "cloudctl/time"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	NO_VALUE string = "-"
)

type ingressRule struct {
	portRange *string
	protocol  *string
	source    *string
	sgId      *string
	desc      *string
}
type egressRule struct {
	portRange *string
	protocol  *string
	source    *string
	sgId      *string
	desc      *string
}
type instanceSGSummary struct {
	groupIds     []*string
	ingressRules []*ingressRule
	egressRules  []*egressRule
}

type instanceDetail struct {
	platform   *string
	amiId      *string
	monitor    *string
	osdetails  *string
	launchTime *time.Time
}
type instanceSummary struct {
	id           *string
	publicIp     *string
	publicIpDNS  *string
	az           *string
	privateIp    *string
	privateIpDNS *string
	state        *string
	typee        *string
	vpcId        *string
	subnetId     *string
	iamroleArn   *string
	launchTime   *time.Time
}

type volumeAttachment struct {
	id                  *string
	time                *time.Time
	deleteOnTermination *bool
	device              *string
	state               *string
}

type instanceVolume struct {
	creationTime *time.Time
	size         *int64
	isEncrypt    *bool
	kmsKey       *string
	state        *string
	attachments  []*volumeAttachment
}

type instanceNetworkinterface struct {
	id                  *string
	description         *string
	privateIpv4Add      *string
	privateIpv4DNS      *string
	publicIpv4Add       *string
	publicIpv4DNS       *string
	attachTime          *time.Time
	attachStatus        *string
	vpcId               *string
	subnetId            *string
	deleteOnTermination *bool
	securityGroups      *[]*string
}

type instanceDefinition struct {
	summary           *instanceSummary
	detail            *instanceDetail
	volumes           []*instanceVolume
	sgSummary         *instanceSGSummary
	networkInterfaces []*instanceNetworkinterface
	err               error
}

type instanceListOutput struct {
	instancesByState map[string][]*instanceSummary
	err              *aws.ErrorInfo
}

func (summary *instanceSummary) setIAMProfileARN(profile *ec2.IamInstanceProfile) *instanceSummary {
	novalue := NO_VALUE
	summary.iamroleArn = &novalue
	if profile != nil {
		summary.iamroleArn = profile.Arn
	}
	return summary
}

// we can't creatre ec2 instance without VPC but termiate/shutdown instance doesn't return these details
func (summary *instanceSummary) SetNetworkDetail(vpcId, subnetId *string) *instanceSummary {
	novalue := NO_VALUE
	summary.vpcId = &novalue
	summary.subnetId = &novalue
	if vpcId != nil {
		summary.vpcId = vpcId
	}
	if subnetId != nil {
		summary.subnetId = subnetId
	}

	return summary
}
func (summary *instanceSummary) setPublicAddr(publicIp, publicDnsName *string) *instanceSummary {
	novalue := NO_VALUE
	summary.publicIp = &novalue
	summary.publicIpDNS = &novalue
	if publicIp != nil {
		summary.publicIp = publicIp
	}
	if publicDnsName != nil {
		summary.publicIpDNS = publicDnsName
	}
	return summary
}

func (summary *instanceSummary) setPrivateAddr(privateIp, privateDnsName *string) *instanceSummary {
	novalue := NO_VALUE
	summary.privateIp = &novalue
	summary.privateIpDNS = &novalue
	if privateIp != nil {
		summary.privateIp = privateIp
	}
	if privateDnsName != nil {
		summary.privateIpDNS = privateDnsName
	}
	return summary
}

func newInstanceSummary(instance *ec2.Instance, tz *ctltime.Timezone) *instanceSummary {
	instanceSummary := &instanceSummary{
		id:         instance.InstanceId,
		az:         instance.Placement.AvailabilityZone,
		state:      instance.State.Name,
		typee:      instance.InstanceType,
		launchTime: tz.AdaptTimezone(instance.LaunchTime),
	}
	instanceSummary.setIAMProfileARN(instance.IamInstanceProfile)
	instanceSummary.SetNetworkDetail(instance.VpcId, instance.SubnetId)
	instanceSummary.setPublicAddr(instance.PublicIpAddress, instance.PublicDnsName)
	instanceSummary.setPrivateAddr(instance.PrivateIpAddress, instance.PrivateDnsName)
	return instanceSummary
}

func newInstanceDetail(instance *ec2.Instance) *instanceDetail {
	platform := NO_VALUE
	if instance.Platform != nil {
		platform = *instance.Platform
	}
	return &instanceDetail{
		platform:   &platform,
		amiId:      instance.ImageId,
		monitor:    instance.Monitoring.State,
		osdetails:  instance.PlatformDetails,
		launchTime: instance.LaunchTime,
	}
}

func newInstanceVolume(volume *ec2.Volume) *instanceVolume {
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
		creationTime: volume.CreateTime,
		size:         volume.Size,
		isEncrypt:    volume.Encrypted,
		kmsKey:       &kmsKey,
		state:        volume.State,
	}
}
func newSecurityIngressRules(securityGroupId, securityGroupName, securityGroupDescription string, ingressPermissions []*ec2.IpPermission) (ingressRules []*ingressRule) {
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
				portRange: &portRange,
				protocol:  &protocol,
				source:    userIdGroupPair.GroupId,
				sgId:      &sgIdWithName,
				desc:      description,
			})
		}

		for _, ipRange := range permission.IpRanges {
			description := ipRange.Description
			if description == nil {
				description = &securityGroupDescription
			}
			ingressRules = append(ingressRules, &ingressRule{
				portRange: &portRange,
				protocol:  &protocol,
				source:    ipRange.CidrIp,
				sgId:      &sgIdWithName,
				desc:      description,
			})
		}

		for _, ipRange := range permission.Ipv6Ranges {
			description := ipRange.Description
			if description == nil {
				description = &securityGroupDescription
			}
			ingressRules = append(ingressRules, &ingressRule{
				portRange: &portRange,
				protocol:  &protocol,
				source:    ipRange.CidrIpv6,
				sgId:      &sgIdWithName,
				desc:      description,
			})
		}
	}
	return
}

func newSecurityEgressRules(securityGroupId, securityGroupName, securityGroupDescription string, egressPermissions []*ec2.IpPermission) (egressRules []*egressRule) {
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
				portRange: &portRange,
				protocol:  &protocol,
				source:    ipRange.CidrIp,
				sgId:      &sgIdWithName,
				desc:      description,
			})
		}

		for _, ipRange := range rule.Ipv6Ranges {
			description := ipRange.Description
			if ipRange.Description == nil {
				description = &securityGroupDescription
			}
			egressRules = append(egressRules, &egressRule{
				portRange: &portRange,
				protocol:  &protocol,
				source:    ipRange.CidrIpv6,
				sgId:      &sgIdWithName,
				desc:      description,
			})
		}
	}
	return
}
func newVolumeAttachment(attachment *ec2.VolumeAttachment) *volumeAttachment {
	return &volumeAttachment{
		id:                  attachment.VolumeId,
		time:                attachment.AttachTime,
		deleteOnTermination: attachment.DeleteOnTermination,
		device:              attachment.Device,
		state:               attachment.State,
	}
}

func newInstanceNetworkSummary(eni *ec2.InstanceNetworkInterface) *instanceNetworkinterface {
	publicIpV4Address := NO_VALUE
	publicIpV4DNS := NO_VALUE
	privateIpV4Address := NO_VALUE
	privateIpV4DNS := NO_VALUE
	if eni.Association != nil {
		publicIpV4Address = *eni.Association.PublicIp
		publicIpV4DNS = *eni.Association.PublicDnsName
	}
	if eni.PrivateDnsName != nil {
		privateIpV4DNS = *eni.PrivateDnsName
	}
	if eni.PrivateIpAddress != nil {
		privateIpV4Address = *eni.PrivateIpAddress
	}
	sgIdWithNames := []*string{}
	for _, sg := range eni.Groups {
		o := fmt.Sprintf("%s(%s)", *sg.GroupId, *sg.GroupName)
		sgIdWithNames = append(sgIdWithNames, &o)
	}
	return &instanceNetworkinterface{
		id:                  eni.NetworkInterfaceId,
		description:         eni.Description,
		privateIpv4Add:      &privateIpV4Address,
		privateIpv4DNS:      &privateIpV4DNS,
		publicIpv4Add:       &publicIpV4Address,
		publicIpv4DNS:       &publicIpV4DNS,
		attachTime:          eni.Attachment.AttachTime,
		attachStatus:        eni.Status,
		vpcId:               eni.VpcId,
		subnetId:            eni.SubnetId,
		deleteOnTermination: eni.Attachment.DeleteOnTermination,
		securityGroups:      &sgIdWithNames,
	}

}

func newInstanceDefinition() *instanceDefinition {
	return &instanceDefinition{}
}

func (def *instanceDefinition) SetVolumeSummary(volumes []*instanceVolume) *instanceDefinition {
	def.volumes = volumes
	return def
}

func (def *instanceDefinition) SetInstanceSummary(summary *instanceSummary) *instanceDefinition {
	def.summary = summary
	return def
}
func (def *instanceDefinition) SetInstanceDetail(detail *instanceDetail) *instanceDefinition {
	def.detail = detail
	return def
}
func (def *instanceDefinition) SetSecurityGroupSummary(sgSummary *instanceSGSummary) *instanceDefinition {
	def.sgSummary = sgSummary
	return def
}

func (def *instanceDefinition) SetNetworkInterfaces(interfaces []*instanceNetworkinterface) *instanceDefinition {
	def.networkInterfaces = interfaces
	return def
}
