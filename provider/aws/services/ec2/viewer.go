package ec2

import (
	"cloudctl/viewer"
	"fmt"
	"strings"
)

var (
	instanceListTableHeader = viewer.Row{
		"Id",
		"Type",
		"Az",
		"PublicIp",
		"PrivateIp",
		"vpc",
		"subnet",
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
	data := o.(*instanceListOutput)

	if data.err != nil {
		erroViewer := viewer.NewErrorViewer()
		erroViewer.SetErrorType(data.err.ErrorType)
		erroViewer.SetErrorMessage(data.err.Err.Error())
		return erroViewer
	}

	compoundViewer := viewer.NewCompoundViewer()
	for state, instanceSummaries := range data.instancesByState {
		tViewer := viewer.NewTableViewer()
		tViewer.AddHeader(instanceListTableHeader)
		tViewer.SetTitle(fmt.Sprintf("Instances[%s]", state))
		for _, instance := range instanceSummaries {
			tViewer.AddRow(viewer.Row{
				*instance.id,
				*instance.typee,
				*instance.az,
				*instance.publicIp,
				*instance.privateIp,
				*instance.vpcId,
				*instance.subnetId,
				*instance.launchTime,
			})
		}
		compoundViewer.AddTableViewer(tViewer)
	}
	return compoundViewer
}

func instanceInfoViewer(o interface{}) viewer.Viewer {
	cTviewer := viewer.NewCompoundViewer()

	instance := o.(*instanceDefinition)

	cTviewer.AddTableViewer(renderInstanceSummary(instance.summary))
	cTviewer.AddTableViewer(renderInstanceDetails(instance.detail))
	cTviewer.AddTableViewer(renderInstanceSgSummaryInbound(instance.sgSummary))
	cTviewer.AddTableViewer(renderInstanceSgSummaryOutbound(instance.sgSummary))
	cTviewer.AddTableViewer(renderInstanceVolumeSummary(instance.volumes))
	cTviewer.AddTableViewer(renderInstanceNetworkSummary(instance.networkInterfaces))

	return cTviewer
}

func renderInstanceSummary(o *instanceSummary) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Summary")
	tViewer.AddHeader(instanceSummaryTableHeader)

	tViewer.AddRow(viewer.Row{
		*o.id,
		*o.typee,
		*o.state,
		*o.publicIp,
		*o.publicIpDNS,
		*o.privateIp,
		*o.privateIpDNS,
		*o.vpcId,
		*o.subnetId,
		*o.iamroleArn,
	})
	return tViewer
}

func renderInstanceDetails(o *instanceDetail) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Details")
	tViewer.AddHeader(instanceDetailsTableHeader)

	tViewer.AddRow(viewer.Row{
		*o.platform,
		*o.amiId,
		*o.monitor,
		*o.osdetails,
		*o.launchTime,
	})
	return tViewer
}

func renderInstanceSgSummaryInbound(o *instanceSGSummary) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Ingress Rules")
	tViewer.AddHeader(instanceSecurityGroupInboundSummaryTableHeader)

	for _, inboundRule := range o.ingressRules {
		tViewer.AddRow(viewer.Row{
			*inboundRule.portRange,
			*inboundRule.protocol,
			*inboundRule.source,
			*inboundRule.sgId,
			*inboundRule.desc,
		})
	}
	return tViewer
}

func renderInstanceSgSummaryOutbound(o *instanceSGSummary) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Egress Rules")
	tViewer.AddHeader(instanceSecurityGroupOutboundSummaryTableHeader)
	for _, outboundRule := range o.egressRules {
		tViewer.AddRow(viewer.Row{
			*outboundRule.portRange,
			*outboundRule.protocol,
			*outboundRule.source,
			*outboundRule.sgId,
			*outboundRule.desc,
		})
	}
	return tViewer
}

func renderInstanceVolumeSummary(volumes []*instanceVolume) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Volumes")
	tViewer.AddHeader(instanceVolumeTableHeader)

	for _, volume := range volumes {
		for _, attachment := range volume.attachments {
			tViewer.AddRow(viewer.Row{
				*attachment.id,
				*attachment.device,
				*volume.size,
				*attachment.state,
				*attachment.time,
				volume.isEncrypt,
				*volume.kmsKey,
				*attachment.deleteOnTermination,
			})
		}
	}

	return tViewer
}

func renderInstanceNetworkSummary(instanceNetworkinterfaces []*instanceNetworkinterface) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Networks")
	tViewer.AddHeader(instanceNetworkSummaryTableHeader)

	for _, networkinterface := range instanceNetworkinterfaces {

		securityGroupsArr := []string{}
		for _, sg := range *networkinterface.securityGroups {
			securityGroupsArr = append(securityGroupsArr, *sg)
		}

		tViewer.AddRow(viewer.Row{
			*networkinterface.id,
			*networkinterface.description,
			*networkinterface.privateIpv4Add,
			*networkinterface.privateIpv4DNS,
			*networkinterface.publicIpv4Add,
			*networkinterface.publicIpv4DNS,
			*networkinterface.attachTime,
			*networkinterface.attachStatus,
			*networkinterface.vpcId,
			*networkinterface.subnetId,
			*networkinterface.deleteOnTermination,
			strings.Join(securityGroupsArr, "\n"),
		})
	}

	return tViewer
}
