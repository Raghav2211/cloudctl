package security

// Report is one resource's full set of findings from `ctl aws <service>
// security <id>` — empty Findings means every rule checked passed, not
// that nothing was checked (see Viewer, which renders that distinction
// explicitly).
type Report struct {
	ResourceID string
	Findings   []Finding
}
