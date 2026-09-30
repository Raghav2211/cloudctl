package nlquery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudctl/ai"
)

func classifyWithResponse(t *testing.T, modelResponse string) (*Intent, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"response":` + jsonQuote(modelResponse) + `}`))
	}))
	defer srv.Close()

	client := ai.NewClient(srv.URL, "test-model")
	return Classify(context.Background(), client, "some query")
}

// jsonQuote is a tiny helper so test fixtures can embed arbitrary text
// (including braces and quotes) as a JSON string literal without pulling in
// encoding/json just for this.
func jsonQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}

func TestClassify_ValidFind(t *testing.T) {
	intent, err := classifyWithResponse(t, `{"command":"find","subcommand":"","flags":{"concept":"database"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "find" || intent.Flags["concept"] != "database" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

func TestClassify_ValidEC2LS(t *testing.T) {
	intent, err := classifyWithResponse(t, `{"command":"ec2","subcommand":"ls","flags":{"state":"running","az":"ap-south-1a"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "ec2" || intent.Subcommand != "ls" || intent.Flags["state"] != "running" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

func TestClassify_ValidChanges(t *testing.T) {
	intent, err := classifyWithResponse(t, `{"command":"changes","subcommand":"","flags":{"since":"2h","resource":"i-abc123"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "changes" || intent.Flags["since"] != "2h" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

func TestClassify_ExplicitlyUnsupported(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"","subcommand":"","flags":{},"unsupported_reason":"no ALB service exists yet"}`)
	if err == nil {
		t.Fatal("expected an error for an explicitly unsupported query, got nil")
	}
	if !strings.Contains(err.Error(), "no ALB service exists yet") {
		t.Errorf("expected the unsupported reason to surface, got %v", err)
	}
}

func TestClassify_UnknownCommand_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"iam","subcommand":"ls","flags":{}}`)
	if err == nil {
		t.Fatal("expected an error for a command outside the whitelist, got nil")
	}
}

func TestClassify_UnknownEC2Flag_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"ec2","subcommand":"ls","flags":{"instance-type":"t3.micro"}}`)
	if err == nil {
		t.Fatal("expected an error for a flag outside the ec2 ls whitelist, got nil")
	}
}

func TestClassify_UnknownEC2State_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"ec2","subcommand":"ls","flags":{"state":"deleted"}}`)
	if err == nil {
		t.Fatal("expected an error for a state value outside the whitelist, got nil")
	}
}

func TestClassify_EC2DefSubcommand_Rejected(t *testing.T) {
	// "def" is a real cloudctl subcommand elsewhere, but not one this
	// package's ec2 whitelist allows — confirms subcommands aren't
	// accepted just because they exist somewhere in the CLI.
	_, err := classifyWithResponse(t, `{"command":"ec2","subcommand":"def","flags":{}}`)
	if err == nil {
		t.Fatal("expected an error for an unwhitelisted ec2 subcommand, got nil")
	}
}

func TestClassify_UnknownConcept_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"find","subcommand":"","flags":{"concept":"loadbalancer"}}`)
	if err == nil {
		t.Fatal("expected an error for a concept outside the whitelist, got nil")
	}
}

func TestClassify_InvalidChangesSinceDuration_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"changes","subcommand":"","flags":{"since":"two hours"}}`)
	if err == nil {
		t.Fatal("expected an error for an unparseable duration, got nil")
	}
}

func TestClassify_UnknownChangesFlag_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, `{"command":"changes","subcommand":"","flags":{"account":"123456789012"}}`)
	if err == nil {
		t.Fatal("expected an error for a flag outside the changes whitelist, got nil")
	}
}

func TestClassify_NoJSONInResponse_Rejected(t *testing.T) {
	_, err := classifyWithResponse(t, "I'm sorry, I don't understand the request.")
	if err == nil {
		t.Fatal("expected an error when the model's response contains no JSON object, got nil")
	}
}

func TestClassify_ThinkingModelWrapsJSONInProse(t *testing.T) {
	// Local "thinking" models routinely emit reasoning text around the
	// answer despite being told to respond with only JSON — confirm
	// extraction still finds the object.
	response := "<think>the user wants running instances</think>\nHere you go:\n" +
		`{"command":"ec2","subcommand":"ls","flags":{"state":"running"}}` + "\nHope that helps!"
	intent, err := classifyWithResponse(t, response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "ec2" || intent.Flags["state"] != "running" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

// TestClassify_CombinedCommandString_Normalized is a regression test: local
// models sometimes write {"command":"ec2 ls",...} despite Vocabulary
// explicitly saying command/subcommand are separate fields. This must still
// resolve to the ec2 ls intent rather than being rejected as an unknown
// command.
func TestClassify_CombinedCommandString_Normalized(t *testing.T) {
	intent, err := classifyWithResponse(t, `{"command":"ec2 ls","subcommand":"","flags":{"state":"running"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "ec2" || intent.Subcommand != "ls" || intent.Flags["state"] != "running" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

// TestClassify_CombinedCommandString_DoesNotOverrideRealSubcommand confirms
// the normalization never fires when Subcommand is already populated —
// it must only patch the specific "combined into command" mistake, not
// rewrite an intent the model already encoded correctly.
func TestClassify_CombinedCommandString_DoesNotOverrideRealSubcommand(t *testing.T) {
	intent, err := classifyWithResponse(t, `{"command":"ec2","subcommand":"ls","flags":{"state":"running"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if intent.Command != "ec2" || intent.Subcommand != "ls" {
		t.Errorf("unexpected intent: %+v", intent)
	}
}

func TestIntent_Describe(t *testing.T) {
	i := &Intent{Command: "ec2", Subcommand: "ls", Flags: map[string]string{"state": "running", "has_public_ip": "true"}}
	got := i.Describe()
	if !strings.HasPrefix(got, "ec2 ls") {
		t.Errorf("expected description to start with 'ec2 ls', got %q", got)
	}
	if !strings.Contains(got, "--state=running") || !strings.Contains(got, "--has-public-ip=true") {
		t.Errorf("expected flags to be rendered as --flag=value with underscores dashed, got %q", got)
	}
}
