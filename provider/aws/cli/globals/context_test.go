package globals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestContextsFile(t *testing.T) {
	t.Helper()
	t.Setenv("CLOUDCTL_CONTEXTS_FILE", filepath.Join(t.TempDir(), "contexts.json"))
}

func TestResolveSessionDefaults_EmptyFlagsFillFromActiveContext(t *testing.T) {
	newTestContextsFile(t)
	saveContextsFile(contextsFile{Current: defaultContextName, Contexts: map[string]contextEntry{
		defaultContextName: {Region: "eu-west-1", Profile: "prod"},
	}})

	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()

	if cf.Region != "eu-west-1" {
		t.Errorf("expected Region to be filled from the active context, got %q", cf.Region)
	}
	if cf.Profile != "prod" {
		t.Errorf("expected Profile to be filled from the active context, got %q", cf.Profile)
	}
}

func TestResolveSessionDefaults_ExplicitFlagOverridesAndPersists(t *testing.T) {
	newTestContextsFile(t)
	saveContextsFile(contextsFile{Current: defaultContextName, Contexts: map[string]contextEntry{
		defaultContextName: {Region: "eu-west-1", Profile: "prod"},
	}})

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

func TestResolveSessionDefaults_NoPersistedContextIsNoop(t *testing.T) {
	newTestContextsFile(t)

	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()

	if cf.Region != "" || cf.Profile != "" {
		t.Errorf("expected empty flags to stay empty with no persisted context, got Region=%q Profile=%q", cf.Region, cf.Profile)
	}
}

func TestResolveSessionDefaults_NeverPersistsCredentials(t *testing.T) {
	newTestContextsFile(t)

	cf := &AWSCLIFlag{Region: "eu-west-1", AccessKey: "AKIA...", SecretKey: "supersecret"}
	cf.ResolveSessionDefaults()

	data, err := os.ReadFile(contextsFilePath())
	if err != nil {
		t.Fatalf("reading contexts file: %v", err)
	}
	if strings.Contains(string(data), "AKIA") || strings.Contains(string(data), "supersecret") {
		t.Fatalf("expected credentials to never be written to the contexts file, got: %s", data)
	}
}

func TestResolveSessionDefaults_UsesNonDefaultActiveContext(t *testing.T) {
	newTestContextsFile(t)
	saveContextsFile(contextsFile{Current: "staging", Contexts: map[string]contextEntry{
		"staging": {Region: "ap-south-1", Profile: "staging-profile"},
		"default": {Region: "eu-west-1", Profile: "prod"},
	}})

	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()

	if cf.Region != "ap-south-1" || cf.Profile != "staging-profile" {
		t.Errorf("expected the active (non-default) context to resolve, got Region=%q Profile=%q", cf.Region, cf.Profile)
	}
}

func TestListContexts_MarksActiveContext(t *testing.T) {
	newTestContextsFile(t)
	SaveContext("staging", "ap-south-1", "staging-profile")
	SaveContext("prod", "eu-west-1", "prod-profile")
	if err := UseContext("prod"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	contexts := ListContexts()
	if len(contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(contexts))
	}
	for _, c := range contexts {
		if c.Name == "prod" && !c.Active {
			t.Error("expected prod to be marked active")
		}
		if c.Name == "staging" && c.Active {
			t.Error("expected staging to not be marked active")
		}
	}
}

func TestUseContext_UnknownNameReturnsError(t *testing.T) {
	newTestContextsFile(t)
	SaveContext("staging", "ap-south-1", "staging-profile")

	if err := UseContext("does-not-exist"); err == nil {
		t.Fatal("expected an error switching to an unknown context, got nil")
	}
}

func TestSaveContext_DoesNotSwitchActiveContext(t *testing.T) {
	newTestContextsFile(t)
	SaveContext("staging", "ap-south-1", "staging-profile")
	if err := UseContext("staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	SaveContext("prod", "eu-west-1", "prod-profile")

	for _, c := range ListContexts() {
		if c.Name == "staging" && !c.Active {
			t.Error("expected staging to remain active after saving an unrelated context")
		}
		if c.Name == "prod" && c.Active {
			t.Error("expected saving prod to not automatically activate it")
		}
	}
}

func TestRemoveContext_ClearsActiveWhenRemovingCurrent(t *testing.T) {
	newTestContextsFile(t)
	SaveContext("staging", "ap-south-1", "staging-profile")
	if err := UseContext("staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := RemoveContext("staging"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ListContexts()) != 0 {
		t.Fatalf("expected no contexts left, got %v", ListContexts())
	}

	// The next resolution should fall back to a fresh implicit "default"
	// context rather than erroring or reusing the removed context's values.
	cf := &AWSCLIFlag{}
	cf.ResolveSessionDefaults()
	if cf.Region != "" || cf.Profile != "" {
		t.Errorf("expected a fresh default context after removing the active one, got Region=%q Profile=%q", cf.Region, cf.Profile)
	}
}

func TestRemoveContext_UnknownNameReturnsError(t *testing.T) {
	newTestContextsFile(t)

	if err := RemoveContext("does-not-exist"); err == nil {
		t.Fatal("expected an error removing an unknown context, got nil")
	}
}
