package globals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestSessionFile(t *testing.T) {
	t.Helper()
	t.Setenv("CLOUDCTL_SESSION_FILE", filepath.Join(t.TempDir(), "session.json"))
}

func TestResolveSessionDefaults_EmptyFlagsFillFromPersistedSession(t *testing.T) {
	newTestSessionFile(t)
	saveSessionState(sessionState{Region: "eu-west-1", Profile: "prod"})

	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()

	if cf.Region != "eu-west-1" {
		t.Errorf("expected Region to be filled from the persisted session, got %q", cf.Region)
	}
	if cf.Profile != "prod" {
		t.Errorf("expected Profile to be filled from the persisted session, got %q", cf.Profile)
	}
}

func TestResolveSessionDefaults_ExplicitFlagOverridesAndPersists(t *testing.T) {
	newTestSessionFile(t)
	saveSessionState(sessionState{Region: "eu-west-1", Profile: "prod"})

	cf := &AWSCLIFlag{Region: "us-east-1"}
	cf.ResolveSessionDefaults()

	if cf.Region != "us-east-1" {
		t.Errorf("expected explicit Region flag to win, got %q", cf.Region)
	}

	// A second, unrelated invocation with no flags set should now pick up
	// the just-saved region, confirming the explicit flag was persisted.
	next := &AWSCLIFlag{}
	next.ResolveSessionDefaults()
	if next.Region != "us-east-1" {
		t.Errorf("expected the explicit region to become the new saved default, got %q", next.Region)
	}
	if next.Profile != "prod" {
		t.Errorf("expected the untouched profile to remain saved, got %q", next.Profile)
	}
}

func TestResolveSessionDefaults_NoPersistedSessionIsNoop(t *testing.T) {
	newTestSessionFile(t)

	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()

	if cf.Region != "" || cf.Profile != "" {
		t.Errorf("expected empty flags to stay empty with no persisted session, got Region=%q Profile=%q", cf.Region, cf.Profile)
	}
}

func TestResolveSessionDefaults_NeverPersistsCredentials(t *testing.T) {
	newTestSessionFile(t)

	cf := &AWSCLIFlag{Region: "eu-west-1", AccessKey: "AKIA...", SecretKey: "supersecret"}
	cf.ResolveSessionDefaults()

	data, err := os.ReadFile(sessionStatePath())
	if err != nil {
		t.Fatalf("reading session file: %v", err)
	}
	if strings.Contains(string(data), "AKIA") || strings.Contains(string(data), "supersecret") {
		t.Fatalf("expected credentials to never be written to the session file, got: %s", data)
	}
}
