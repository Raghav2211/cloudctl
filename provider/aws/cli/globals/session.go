package globals

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// sessionState is the small set of CLI flags persisted across invocations in
// the same environment, so a user isn't forced to repeat --region/--profile
// on every command. Credentials (access key/secret/session token) are
// deliberately never persisted here — writing raw AWS credentials to a
// plaintext file on disk is a real security regression for a convenience
// gain; --profile/env/~/.aws already cover that need.
type sessionState struct {
	Region  string `json:"region,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// sessionStatePath returns ~/.cloudctl/session.json, or the value of
// CLOUDCTL_SESSION_FILE if set — mirrors snapshot.DefaultPath's env-override
// convention (used by tests, and by anyone who wants it elsewhere).
func sessionStatePath() string {
	if p := os.Getenv("CLOUDCTL_SESSION_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cloudctl", "session.json")
}

// loadSessionState reads the persisted region/profile, returning a zero
// value (not an error) if the file doesn't exist yet or can't be parsed —
// this is best-effort convenience, never something a command should fail
// over.
func loadSessionState() sessionState {
	data, err := os.ReadFile(sessionStatePath())
	if err != nil {
		return sessionState{}
	}
	var s sessionState
	if err := json.Unmarshal(data, &s); err != nil {
		return sessionState{}
	}
	return s
}

// saveSessionState persists region/profile for future invocations,
// best-effort — a write failure (e.g. a read-only home directory) is
// silently ignored rather than failing the command that triggered it.
func saveSessionState(s sessionState) {
	path := sessionStatePath()
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// ResolveSessionDefaults fills Region/Profile from the last persisted
// session when the flag left them empty, then persists whatever ends up in
// effect — an explicit flag always wins and becomes the new saved default.
// Called once, from provider/aws.NewCredentialConfig, so every AWS CLI
// command gets this automatically without repeating the wiring at each
// command's own call site.
func (cf *AWSCLIFlag) ResolveSessionDefaults() {
	state := loadSessionState()
	changed := false

	if cf.Region == "" {
		cf.Region = state.Region
	} else if cf.Region != state.Region {
		state.Region = cf.Region
		changed = true
	}

	if cf.Profile == "" {
		cf.Profile = state.Profile
	} else if cf.Profile != state.Profile {
		state.Profile = cf.Profile
		changed = true
	}

	if changed {
		saveSessionState(state)
	}
}
