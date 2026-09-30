package changes

import (
	"strings"
	"testing"
)

func TestBuildDescription_FullyPopulated(t *testing.T) {
	name := "AuthorizeSecurityGroupIngress"
	source := "ec2.amazonaws.com"
	user := "alice"
	c := &change{eventName: &name, eventSource: &source, username: &user, resources: []string{"sg-0123"}}

	got := buildDescription(c)
	for _, want := range []string{"alice", "AuthorizeSecurityGroupIngress", "sg-0123", "ec2"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected description %q to contain %q", got, want)
		}
	}
	if strings.Contains(got, "amazonaws.com") {
		t.Errorf("expected the .amazonaws.com suffix to be stripped, got %q", got)
	}
}

func TestBuildDescription_MissingFieldsDegradeGracefully(t *testing.T) {
	c := &change{}
	got := buildDescription(c)
	if !strings.Contains(got, "someone") || !strings.Contains(got, "an unrecorded action") {
		t.Errorf("expected graceful placeholders for missing fields, got %q", got)
	}
}
