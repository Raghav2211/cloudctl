package ec2

import (
	"cloudctl/viewer"
	"strings"
)

var (
	instanceListTableHeader = viewer.Row{
		"Id",
		"Type",
		"State",
		"Az",
		"PublicIp",
		"PrivateIp",
		"LaunchAt",
	}
	instanceSummaryTableHeader = viewer.Row{
		"Id",
		"Type",
		"State",
		"PublicIp",
		"PublicIpDNS",
		"PrivateIp",
		"PrivateIpDNS",
		"Vpc",
		"Subnet",
		"IAMRoleARN",
	}

	instanceDetailsTableHeader = viewer.Row{
		"Platform",
		"AmiId",
		"Monitoring",
		"OSType",
		"LaunchTime",
	}
	instanceSecurityGroupInboundSummaryTableHeader = viewer.Row{
		"PortRange",
		"Protocol",
		"Source",
		"GroupId",
		"Description",
	}

	instanceSecurityGroupOutboundSummaryTableHeader = viewer.Row{
		"PortRange",
		"Protocol",
		"Destination",
		"GroupId",
		"Description",
	}

	instanceVolumeTableHeader = viewer.Row{
		"Id",
		"DeviceName",
		"Size",
		"Status",
		"Time",
		"Encrypted",
		"KMS",
		"DeleteOntermination",
	}
	instanceNetworkSummaryTableHeader = viewer.Row{
		"id",
		"description",
		"privateIpv4Add",
		"privateIpv4DNS",
		"publicIpv4Add",
		"publicIpv4DNS",
		"attachTime",
		"attachStatus",
		"vpcId",
		"subnetId",
		"deleteOnTermination",
		"securityGroups",
	}
)

func instanceListViewer(o interface{}) viewer.Viewer {
	tViewer := viewer.NewTableViewer()
	tViewer.AddHeader(instanceListTableHeader)
	tViewer.SetTitle("Instances")
	data := o.(*instanceListOutput)
	rows := []viewer.Row{}
	for _, instance := range data.instances {
		rows = append(rows, viewer.Row{
			*instance.id,
			*instance.typee,
			*instance.state,
			*instance.az,
			*instance.publicIp,
			*instance.privateIp,
			*instance.launchTime,
		})
	}
	tViewer.AddRows(rows)
	return tViewer
}

func instanceInfoViewer(o interface{}) viewer.Viewer {
	cTviewer := viewer.NewCompoundTableViewer()

	iSummaryTviewerChan := make(chan *viewer.TableViewer)
	iDetailsTviewerChan := make(chan *viewer.TableViewer)
	iSgInboundTviewerChan := make(chan *viewer.TableViewer)
	iSgOutboundTviewerChan := make(chan *viewer.TableViewer)
	iVolumeTviewerChan := make(chan *viewer.TableViewer)
	iNetworkTviewerChan := make(chan *viewer.TableViewer)
	instance := o.(*instanceDetailView)
	go func() {
		defer close(iSummaryTviewerChan)
		renderInstanceSummary(iSummaryTviewerChan, instance.summary)
	}()
	go func() {
		defer close(iDetailsTviewerChan)
		renderInstanceDetails(iDetailsTviewerChan, instance.detail)
	}()
	go func() {
		defer close(iSgInboundTviewerChan)
		renderInstanceSgSummaryInbound(iSgInboundTviewerChan, instance.sgSummary)
	}()
	go func() {
		defer close(iSgOutboundTviewerChan)
		renderInstanceSgSummaryOutbound(iSgOutboundTviewerChan, instance.sgSummary)
	}()
	go func() {
		defer close(iVolumeTviewerChan)
		renderInstanceVolumeSummary(iVolumeTviewerChan, instance.volumes)
	}()
	go func() {
		defer close(iNetworkTviewerChan)
		renderInstanceNetworkSummary(iNetworkTviewerChan, instance.ntwrkSummary)
	}()
	cTviewer.AddTableViewer(<-iSummaryTviewerChan)
	cTviewer.AddTableViewer(<-iDetailsTviewerChan)
	cTviewer.AddTableViewer(<-iSgInboundTviewerChan)
	cTviewer.AddTableViewer(<-iSgOutboundTviewerChan)
	cTviewer.AddTableViewer(<-iVolumeTviewerChan)
	cTviewer.AddTableViewer(<-iNetworkTviewerChan)
	return cTviewer
}

func renderInstanceSummary(outputChan chan<- *viewer.TableViewer, o *instanceSummary) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Summary")
	tViewer.AddHeader(instanceSummaryTableHeader)

	tViewer.AddRow(viewer.Row{
		o.id,
		o.typee,
		o.state,
		o.publicIp,
		o.publicIpDNS,
		o.privateIp,
		o.privateIpDNS,
		o.vpcId,
		o.subnetId,
		o.iamroleArn,
	})
	outputChan <- tViewer
}

func renderInstanceDetails(outputChan chan<- *viewer.TableViewer, o *instanceDetail) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Details")
	tViewer.AddHeader(instanceDetailsTableHeader)

	tViewer.AddRow(viewer.Row{
		o.platform,
		o.amiId,
		o.monitor,
		o.osdetails,
		o.launchTime,
	})
	outputChan <- tViewer
}

func renderInstanceSgSummaryInbound(outputChan chan<- *viewer.TableViewer, o *instanceSGSummary) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("SG/Inbound")
	tViewer.AddHeader(instanceSecurityGroupInboundSummaryTableHeader)

	for _, inboundRule := range o.inboundRules {
		tViewer.AddRow(viewer.Row{
			inboundRule.portRange,
			inboundRule.protocol,
			inboundRule.source,
			inboundRule.sgId,
			inboundRule.desc,
		})
	}
	outputChan <- tViewer
}

func renderInstanceSgSummaryOutbound(outputChan chan<- *viewer.TableViewer, o *instanceSGSummary) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("SG/Outbound")
	tViewer.AddHeader(instanceSecurityGroupOutboundSummaryTableHeader)
	for _, outboundRule := range o.outboundRules {
		tViewer.AddRow(viewer.Row{
			outboundRule.portRange,
			outboundRule.protocol,
			outboundRule.source,
			outboundRule.sgId,
			outboundRule.desc,
		})
	}
	outputChan <- tViewer
}

func renderInstanceVolumeSummary(outputChan chan<- *viewer.TableViewer, volumes []*instanceVolume) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Volumes")
	tViewer.AddHeader(instanceVolumeTableHeader)

	for _, volume := range volumes {
		for _, attachment := range volume.attachments {
			tViewer.AddRow(viewer.Row{
				attachment.id,
				attachment.device,
				volume.size,
				attachment.state,
				attachment.time,
				volume.isEncrypt,
				volume.kmsKey,
				attachment.deleteOnTermination,
			})
		}
	}

	outputChan <- tViewer
}

func renderInstanceNetworkSummary(outputChan chan<- *viewer.TableViewer, instanceNetworkinterfaces []*instanceNetworkinterface) {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Networks")
	tViewer.AddHeader(instanceNetworkSummaryTableHeader)

	for _, ntwrkInterface := range instanceNetworkinterfaces {

		securityGroupsArr := []string{}
		for _, sg := range ntwrkInterface.securityGroups {
			securityGroupsArr = append(securityGroupsArr, *sg)
		}

		tViewer.AddRow(viewer.Row{
			ntwrkInterface.id,
			ntwrkInterface.description,
			ntwrkInterface.privateIpv4Add,
			ntwrkInterface.privateIpv4DNS,
			ntwrkInterface.publicIpv4Add,
			ntwrkInterface.publicIpv4DNS,
			ntwrkInterface.attachTime,
			ntwrkInterface.attachStatus,
			ntwrkInterface.vpcId,
			ntwrkInterface.subnetId,
			ntwrkInterface.deleteOnTermination,
			strings.Join(securityGroupsArr, "\n"),
		})
	}

	outputChan <- tViewer
}
