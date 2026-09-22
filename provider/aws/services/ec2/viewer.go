package ec2

import (
	ctlaws "cloudctl/provider/aws"
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
	instanceStatisticsTableHeader = viewer.Row{
		"instanceId",
		"Minimum",
		"Average",
		"Maximum",
		"Status",
	}
)

func instanceListViewer(data *instanceListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compoundViewer := viewer.NewCompoundViewer()
	for state, instanceSummaries := range data.instancesByState {
		tViewer := viewer.NewTableViewer()
		tViewer.AddHeader(instanceListTableHeader)
		tViewer.SetTitle(fmt.Sprintf("Instances[%s]", state))

		// Apply default enhanced styling (now truly generic)
		style := viewer.DefaultTableStyle()
		tViewer.SetStyle(style)

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
		compoundViewer.AddViewer(tViewer)
	}
	return compoundViewer
}

func instanceInfoViewer(instance *instanceDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	cTviewer := viewer.NewCompoundViewer()
	cTviewer.AddViewer(renderInstanceSummary(instance.summary))
	cTviewer.AddViewer(renderInstanceDetails(instance.detail))
	cTviewer.AddViewers(renderInstanceRulesSummary(instance.ruleSummary))
	cTviewer.AddViewer(renderInstanceVolumeSummary(instance.volumesSummary))
	cTviewer.AddViewer(renderInstanceNetworkSummary(instance.networkInterfaces))

	return cTviewer
}

func ec2StatisticsViewer(data *instanceStatisticsListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.AddHeader(instanceStatisticsTableHeader)
	tViewer.SetTitle("Statistics")

	for _, stats := range data.stats {
		if stats.apiError != nil {
			tViewer.AddRow(viewer.Row{
				*stats.instanceId,
				"-",
				"-",
				"-",
				stats.apiError.Err.Error(),
			})
			continue
		}
		tViewer.AddRow(viewer.Row{
			*stats.instanceId,
			*stats.Minimum,
			*stats.Average,
			*stats.Maximum,
			stats.CPUStatus,
		})
	}
	return tViewer
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

func renderInstanceRulesSummary(summary *instanceIngressEgressRuleSummary) []viewer.Viewer {
	viewers := []viewer.Viewer{}
	if summary.apiError != nil {
		viewers = append(viewers, ctlaws.ErrorView(summary.apiError))
	} else {
		viewers = append(viewers, renderInstanceIngressRules(summary.ingressRules))
		viewers = append(viewers, renderInstanceEgressRules(summary.egressRules))
	}
	return viewers
}

func renderInstanceIngressRules(rules []*ingressRule) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Ingress Rules")
	tViewer.AddHeader(instanceSecurityGroupInboundSummaryTableHeader)

	for _, rule := range rules {
		tViewer.AddRow(viewer.Row{
			*rule.portRange,
			*rule.protocol,
			*rule.source,
			*rule.sgId,
			*rule.desc,
		})
	}
	return tViewer
}

func renderInstanceEgressRules(rules []*egressRule) *viewer.TableViewer {

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Egress Rules")
	tViewer.AddHeader(instanceSecurityGroupOutboundSummaryTableHeader)
	for _, rule := range rules {
		tViewer.AddRow(viewer.Row{
			*rule.portRange,
			*rule.protocol,
			*rule.source,
			*rule.sgId,
			*rule.desc,
		})
	}
	return tViewer
}

func renderInstanceVolumeSummary(volumesSummary *instanceVolumeSummary) viewer.Viewer {
	if volumesSummary.apiError != nil {
		return ctlaws.ErrorView(volumesSummary.apiError)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetTitle("Volumes")
	tViewer.AddHeader(instanceVolumeTableHeader)

	for _, volume := range volumesSummary.volumes {
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

func sgExplainViewer(data *sgExplanation, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()
	compound.AddViewer(viewer.FuncViewer(func() {
		fmt.Printf("=== AI Summary for %s (Hypothesis — verify against the raw rules below) ===\n", *data.sgId)
		if data.aiSummary != "" {
			fmt.Println(data.aiSummary)
		} else {
			reason := data.aiSummaryUnavailable
			if reason == "" {
				reason = "not attempted"
			}
			fmt.Printf("summary unavailable: %s\n", reason)
		}
		fmt.Println()
	}))
	compound.AddViewer(renderInstanceIngressRules(data.ingressRules))
	compound.AddViewer(renderInstanceEgressRules(data.egressRules))
	return compound
}
