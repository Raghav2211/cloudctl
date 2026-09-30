package globals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// contextEntry is one named, saved region/profile pair. Credentials
// (access key/secret/session token) are deliberately never persisted here —
// writing raw AWS credentials to a plaintext file on disk is a real
// security regression for a convenience gain; --profile/env/~/.aws already
// cover that need.
type contextEntry struct {
	Region  string `json:"region,omitempty"`
	Profile string `json:"profile,omitempty"`
}

// contextsFile is the on-disk shape of ~/.cloudctl/contexts.json: every
// saved named context, plus which one is currently active. A user who never
// saves a named context still gets --region/--profile persistence: an
// implicit "default" context is created and maintained automatically the
// first time ResolveSessionDefaults runs.
type contextsFile struct {
	Current  string                  `json:"current,omitempty"`
	Contexts map[string]contextEntry `json:"contexts,omitempty"`
}

const defaultContextName = "default"

// contextsFilePath returns ~/.cloudctl/contexts.json, or the value of
// CLOUDCTL_CONTEXTS_FILE if set — mirrors snapshot.DefaultPath's
// env-override convention (used by tests, and by anyone who wants it
// elsewhere).
func contextsFilePath() string {
	if p := os.Getenv("CLOUDCTL_CONTEXTS_FILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".cloudctl", "contexts.json")
}

// loadContextsFile reads the persisted contexts, returning a zero value
// (not an error) if the file doesn't exist yet or can't be parsed — this is
// best-effort convenience, never something a command should fail over.
func loadContextsFile() contextsFile {
	data, err := os.ReadFile(contextsFilePath())
	if err != nil {
		return contextsFile{}
	}
	var f contextsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return contextsFile{}
	}
	return f
}

// saveContextsFile persists the contexts file, best-effort — a write
// failure (e.g. a read-only home directory) is silently ignored rather than
// failing the command that triggered it.
func saveContextsFile(f contextsFile) {
	path := contextsFilePath()
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	data, err := json.Marshal(f)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}

// ResolveSessionDefaults fills Region/Profile from the currently active
// named context when the flag left them empty, then persists whatever ends
// up in effect into that context — an explicit flag always wins and becomes
// the new saved default for the active context. If no context is active yet
// (first-ever run, or the active one was removed), an implicit "default"
// context is created and made active. Called once, from
// provider/aws.NewCredentialConfig, so every AWS CLI command gets this
// automatically without repeating the wiring at each command's own call
// site.
func (cf *AWSCLIFlag) ResolveSessionDefaults() {
	f := loadContextsFile()
	if f.Current == "" {
		f.Current = defaultContextName
	}
	if f.Contexts == nil {
		f.Contexts = map[string]contextEntry{}
	}
	entry := f.Contexts[f.Current]
	changed := false

	if cf.Region == "" {
		cf.Region = entry.Region
	} else if cf.Region != entry.Region {
		entry.Region = cf.Region
		changed = true
	}

	if cf.Profile == "" {
		cf.Profile = entry.Profile
	} else if cf.Profile != entry.Profile {
		entry.Profile = cf.Profile
		changed = true
	}

	if changed {
		f.Contexts[f.Current] = entry
		saveContextsFile(f)
	}
}

// ContextInfo is a named context's saved region/profile, plus whether it's
// currently active — the shape callers outside this package (the `ctl
// context` commands) need, without exposing the on-disk format directly.
type ContextInfo struct {
	Name    string
	Region  string
	Profile string
	Active  bool
}

// ListContexts returns every saved named context, sorted by name, each
// flagged with whether it's the currently active one.
func ListContexts() []ContextInfo {
	f := loadContextsFile()
	names := make([]string, 0, len(f.Contexts))
	for name := range f.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]ContextInfo, 0, len(names))
	for _, name := range names {
		e := f.Contexts[name]
		infos = append(infos, ContextInfo{Name: name, Region: e.Region, Profile: e.Profile, Active: name == f.Current})
	}
	return infos
}

// UseContext switches the active context to name, returning an error if it
// doesn't exist yet — use SaveContext first to create it.
func UseContext(name string) error {
	f := loadContextsFile()
	if _, ok := f.Contexts[name]; !ok {
		return fmt.Errorf("no saved context named %q (known: %s)", name, strings.Join(contextNames(f), ", "))
	}
	f.Current = name
	saveContextsFile(f)
	return nil
}

// SaveContext creates or updates a named context with the given
// region/profile. It does not switch the active context — run `UseContext`
// (i.e. `ctl context use <name>`) separately to activate it, so saving
// never has a surprising side effect on what's currently in use.
func SaveContext(name, region, profile string) {
	f := loadContextsFile()
	if f.Contexts == nil {
		f.Contexts = map[string]contextEntry{}
	}
	f.Contexts[name] = contextEntry{Region: region, Profile: profile}
	saveContextsFile(f)
}

// RemoveContext deletes a named context. Removing the active context clears
// Current — the next command falls back to an implicit "default" context,
// recreated automatically by ResolveSessionDefaults.
func RemoveContext(name string) error {
	f := loadContextsFile()
	if _, ok := f.Contexts[name]; !ok {
		return fmt.Errorf("no saved context named %q", name)
	}
	delete(f.Contexts, name)
	if f.Current == name {
		f.Current = ""
	}
	saveContextsFile(f)
	return nil
}

func contextNames(f contextsFile) []string {
	names := make([]string, 0, len(f.Contexts))
	for name := range f.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
