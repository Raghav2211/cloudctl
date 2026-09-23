package vpc

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
)

var vpcListTableHeader = viewer.Row{
	"VpcId",
	"CIDR",
	"Default",
	"State",
}

func vpcListViewer(data *vpcListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.SetTitle("VPCs")
	tViewer.AddHeader(vpcListTableHeader)
	for _, v := range data.vpcs {
		tViewer.AddRow(viewer.Row{derefStr(v.id), derefStr(v.cidr), v.isDefault, derefStr(v.state)})
	}
	return tViewer
}

func vpcDefinitionViewer(data *vpcDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", derefStr(data.id)))
	if data.aiSummary != "" {
		summaryPanel.SetBody(data.aiSummary)
	} else {
		reason := data.aiSummaryUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		summaryPanel.SetBody("summary unavailable: " + reason)
	}
	compound.AddViewer(summaryPanel)

	// This is genuinely hierarchical data (VPC -> subnets, gateways), so it
	// renders as a real tree (Track H's viewer.Tree) instead of a flat table
	// with a repeated parent-ID column.
	tree := viewer.NewTree(derefStr(data.id)).SetTitle(fmt.Sprintf("VPC Topology (%s)", derefStr(data.cidr)))
	for _, s := range data.subnets {
		classification := "private"
		if s.isPublic {
			classification = "public"
		}
		tree.Child(fmt.Sprintf("subnet %s (%s, %s, %s)", derefStr(s.id), derefStr(s.cidr), derefStr(s.az), classification))
	}
	for _, n := range data.natGateways {
		tree.Child(fmt.Sprintf("nat-gateway %s (subnet %s, %s)", derefStr(n.id), derefStr(n.subnetID), derefStr(n.state)))
	}
	for _, g := range data.internetGateways {
		tree.Child(fmt.Sprintf("internet-gateway %s", derefStr(g.id)))
	}
	compound.AddViewer(tree)

	return compound
}
