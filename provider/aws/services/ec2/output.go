package ec2

import (
	"time"
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

type sgInboundRule struct {
	portRange string
	protocol  string
	source    string
	sgId      string
	desc      string
}
type sgOutboundRule struct {
	portRange string
	protocol  string
	source    string
	sgId      string
	desc      string
}
type instanceSGSummary struct {
	groupIds      []*string
	inboundRules  []*sgInboundRule
	outboundRules []*sgOutboundRule
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

type instanceDetailView struct {
	summary      *instanceSummary
	detail       *instanceDetail
	volumes      []*instanceVolume
	sgSummary    *instanceSGSummary
	ntwrkSummary []*instanceNetworkinterface
}

type instanceListOutput struct {
	instances []*instance
}
