package main

import (
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/drift"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/headless"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
	"github.com/martinhg/capiko-ai/internal/versions"
)

// engramAdvisory returns a non-fatal advisory when capiko manages engram and the
// installed binary is behind the recommended version; "" otherwise. It is a
// package var so tests can stub it without shelling out to engram.
var engramAdvisory = func(store *state.Store) string {
	managed := false
	if store != nil {
		if st, err := store.Load(); err == nil && st.Engram != nil {
			managed = st.Engram.Enabled
		}
	}
	return engram.OutdatedAdvisory(managed, versions.Engram)
}

// syncInputs bundles everything syncCommand needs from the outside world,
// gathered by gatherSyncInputs so tests can stub the environment.
type syncInputs struct {
	host         *copilot.Host
	hostExitCode int   // 0 success, 1 detect error, 2 Copilot not found
	hostErr      error // non-nil only when hostExitCode == 1
	catalog      []skill.Skill
	agents       []agent.Agent
	store        *state.Store
	bkp          *backup.Store
}

// gatherSyncInputs collects the live environment for the sync command. It is a
// package var so tests can stub it without touching the real PATH, home
// directory, or embedded catalog.
var gatherSyncInputs = func() (syncInputs, error) {
	host, exitCode := requireHost(copilot.Detect)
	in := syncInputs{host: host, hostExitCode: exitCode}
	if exitCode != 0 {
		if exitCode == 1 {
			// requireHost swallows the detect error; call again to surface it for
			// exit reporting. Cheap: only on the already-failing path.
			_, err := copilot.Detect()
			in.hostErr = err
		}
		return in, nil // early return; syncCommand handles the exit code
	}

	cat, err := catalog.Load()
	if err != nil {
		return in, err
	}
	in.catalog = cat

	agents, err := catalog.LoadAgents()
	if err != nil {
		return in, err
	}
	in.agents = agents

	store, _ := state.DefaultStore() // nil-tolerant
	in.store = store
	bkp, _ := backup.DefaultStore() // nil-tolerant
	in.bkp = bkp
	return in, nil
}

// runSync is the seam wrapping tui.RunSync, allowing tests to inject failures
// without touching the filesystem or real Copilot host.
var runSync = func(host *copilot.Host, cat []skill.Skill, agents []agent.Agent, store *state.Store, bkp *backup.Store) (int, error) {
	return tui.RunSync(host, cat, agents, store, bkp)
}

// syncCommand writes every catalog skill and agent to disk (overwriting), so
// the installed items match the current catalog exactly. handled is false when
// name is not "sync", so main falls through to the configurator TUI.
//
//	capiko-ai sync [--json] [--auto-repair]
func syncCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "sync" {
		return false, 0, nil
	}
	asJSON, autoRepair, err := parseSyncArgs(args)
	if err != nil {
		return true, 1, err
	}

	in, err := gatherSyncInputs()
	if err != nil {
		return true, 1, err
	}

	if in.hostExitCode != 0 {
		msg := "GitHub Copilot CLI not found"
		if in.hostExitCode == 1 && in.hostErr != nil {
			msg = in.hostErr.Error()
		}
		r := headless.FromReconcileResult("sync", tui.ReconcileResult{}, fmt.Errorf("%s", msg))
		renderSync(out, r, asJSON)
		return true, in.hostExitCode, in.hostErr
	}

	if autoRepair {
		// --auto-repair skips the write only when we can POSITIVELY confirm there
		// is no drift. A nil store means DefaultStore() failed (home dir
		// undetectable) — NOT a clean state — so drift is indeterminable and we
		// must bias toward repairing: fall through to RunSync, matching the
		// behavior of an unconditional sync. Only short-circuit when a real store
		// reports no drift.
		//
		// Scope note: "drift" here is checksum divergence for items recorded in
		// state, plus missing agents and a stale engram entry (see internal/drift).
		// A skill that was never installed is not flagged as drift, so a clean
		// (freshly-seeded) state still syncs because every catalog agent reads as
		// missing.
		if in.store != nil {
			st, _ := in.store.Load()
			staleSkills := drift.Stale(in.catalog, st)
			staleAgents := drift.StaleAgents(in.agents, st)
			staleEngram := drift.StaleEngram(in.host.MCPConfigPath, st)

			if len(staleSkills) == 0 && len(staleAgents) == 0 && !staleEngram {
				// No drift confirmed — render the empty sync result (triggers the
				// "No drift detected" message in RenderText) and exit cleanly
				// without calling RunSync.
				r := headless.FromReconcileResult("sync", tui.ReconcileResult{}, nil)
				renderSync(out, r, asJSON)
				return true, 0, nil
			}
		}
		// Drift detected, or state indeterminable — fall through to RunSync below.
	}

	n, err := runSync(in.host, in.catalog, in.agents, in.store, in.bkp)
	if err != nil {
		r := headless.FromReconcileResult("sync", tui.ReconcileResult{}, err)
		renderSync(out, r, asJSON)
		return true, 1, nil
	}

	// RunSync returns a count, not names. Build the name lists from the catalog
	// (sync writes every catalog item, so all names are "installed").
	skillNames := make([]string, len(in.catalog))
	for i, sk := range in.catalog {
		skillNames[i] = sk.Name
	}
	agentNames := make([]string, len(in.agents))
	for i, a := range in.agents {
		agentNames[i] = a.Name
	}
	result := tui.ReconcileResult{
		InstalledSkills: skillNames,
		InstalledAgents: agentNames,
	}
	// Sanity: TotalChanged should match RunSync's returned count.
	_ = n
	r := headless.FromReconcileResult("sync", result, nil)
	if w := engramAdvisory(in.store); w != "" {
		r.Warnings = append(r.Warnings, w)
	}
	renderSync(out, r, asJSON)
	return true, 0, nil
}

// renderSync writes r to out as JSON or text, ignoring render errors past the
// point of no return (out is typically os.Stdout, where a write failure is
// unrecoverable and not worth surfacing as a command failure).
func renderSync(out io.Writer, r headless.CommandResult, asJSON bool) {
	if asJSON {
		_ = headless.RenderJSON(out, r)
		return
	}
	headless.RenderText(out, r)
}

// parseSyncArgs parses the optional --json and --auto-repair flags.
func parseSyncArgs(args []string) (asJSON, autoRepair bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--auto-repair":
			autoRepair = true
		default:
			return false, false, fmt.Errorf("sync: unknown argument %q", a)
		}
	}
	return asJSON, autoRepair, nil
}
