package tui

// ReconcileResult is the outcome of a headless install or uninstall
// operation: which skills and agents were installed or removed. A single
// command only ever populates one side (installed for `install`, removed for
// `uninstall`); `sync` populates installed only, mirroring the catalog it
// just wrote.
type ReconcileResult struct {
	InstalledSkills []string
	InstalledAgents []string
	RemovedSkills   []string
	RemovedAgents   []string
}

// TotalChanged returns the total number of items installed or removed.
func (r ReconcileResult) TotalChanged() int {
	return len(r.InstalledSkills) + len(r.InstalledAgents) +
		len(r.RemovedSkills) + len(r.RemovedAgents)
}
