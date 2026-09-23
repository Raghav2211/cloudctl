package eks

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"fmt"
	"strings"
)

var clusterListTableHeader = viewer.Row{
	"Name",
}

func clusterListViewer(data *clusterListOutput, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	tViewer := viewer.NewTableViewer()
	tViewer.SetStyle(viewer.DefaultTableStyle())
	tViewer.SetTitle("EKS Clusters")
	tViewer.AddHeader(clusterListTableHeader)
	for _, c := range data.clusters {
		tViewer.AddRow(viewer.Row{derefStr(c.name)})
	}
	return tViewer
}

func clusterDefinitionViewer(data *clusterDefinition, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	compound := viewer.NewCompoundViewer()

	summaryPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Summary for %s (Hypothesis — verify against the data below)", derefStr(data.name)))
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

	recommendationsPanel := viewer.NewPanel().SetTitle(fmt.Sprintf("AI Recommendations for %s (Recommendation — a human must apply these)", derefStr(data.name)))
	if data.aiRecommendations != "" {
		recommendationsPanel.SetBody(data.aiRecommendations)
	} else {
		reason := data.aiRecommendationsUnavailable
		if reason == "" {
			reason = "not attempted"
		}
		recommendationsPanel.SetBody("recommendations unavailable: " + reason)
	}
	compound.AddViewer(recommendationsPanel)

	dataPanel := viewer.NewPanel().SetTitle("Cluster Configuration")
	dataPanel.AddEntry("Version", derefStr(data.version))
	dataPanel.AddEntry("Status", derefStr(data.status))
	dataPanel.AddEntry("Endpoint", derefStr(data.endpoint))
	access := "private-only"
	switch {
	case data.endpointPublicAccess && data.endpointPrivateAccess:
		access = "public and private"
	case data.endpointPublicAccess:
		access = "public-only"
	}
	dataPanel.AddEntry("Endpoint Access", access)
	if data.endpointPublicAccess && len(data.publicAccessCidrs) > 0 {
		dataPanel.AddEntry("Public Access CIDRs", strings.Join(data.publicAccessCidrs, ", "))
	}
	logging := "none enabled"
	if len(data.enabledClusterLogTypes) > 0 {
		logging = strings.Join(data.enabledClusterLogTypes, ", ")
	}
	dataPanel.AddEntry("Cluster Logging", logging)
	compound.AddViewer(dataPanel)

	// Cluster -> node groups is hierarchical, so it renders as a real tree
	// (Track H's viewer.Tree), same as VPC's subnet topology.
	if len(data.nodeGroups) > 0 {
		tree := viewer.NewTree(derefStr(data.name)).SetTitle("Node Groups")
		for _, ng := range data.nodeGroups {
			tree.Child(fmt.Sprintf("%s: %s, %s, instances=%v, desired=%d min=%d max=%d",
				derefStr(ng.name), derefStr(ng.status), ng.capacityType, ng.instanceTypes,
				derefInt32(ng.desiredSize), derefInt32(ng.minSize), derefInt32(ng.maxSize)))
		}
		compound.AddViewer(tree)
	}

	return compound
}
