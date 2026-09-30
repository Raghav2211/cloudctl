package identity

// whoami is `ctl whoami aws`'s output: who the currently active AWS
// credentials resolve to, and where they came from — answering "is this
// even going to work" before running a real command against the wrong
// account, region, or an expired credential.
type whoami struct {
	account          *string
	arn              *string
	userID           *string
	region           string
	credentialSource string
}

func newWhoami(account, arn, userID *string, region, credentialSource string) *whoami {
	return &whoami{
		account:          account,
		arn:              arn,
		userID:           userID,
		region:           region,
		credentialSource: credentialSource,
	}
}
