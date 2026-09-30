package identity

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
)

func whoamiViewer(data *whoami, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}

	panel := viewer.NewPanel().SetTitle("AWS Identity")
	panel.AddEntry("Account", derefStr(data.account))
	panel.AddEntry("ARN", derefStr(data.arn))
	panel.AddEntry("User ID", derefStr(data.userID))
	panel.AddEntry("Region", data.region)
	credentialSource := data.credentialSource
	if credentialSource == "" {
		credentialSource = "unknown"
	}
	panel.AddEntry("Credential Source", credentialSource)
	return panel
}

func derefStr(s *string) string {
	if s == nil {
		return "-"
	}
	return *s
}
