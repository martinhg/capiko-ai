package main

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/doctor"
	"github.com/martinhg/capiko-ai/internal/drift"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// gatherDoctorInputs collects the live environment for the doctor report. It is a
// package var so tests can stub it without touching the real PATH, home, or state.
var gatherDoctorInputs = func() doctor.Inputs {
	in := doctor.Inputs{Env: sysinfo.Detect(), Now: time.Now()}

	host, _ := copilot.Detect()
	in.CopilotHost = host

	store, err := state.DefaultStore()
	if err != nil {
		return in // state disabled; the State-file check reports "no managed install"
	}
	st, err := store.Load()
	if err != nil {
		in.StateErr = err
		return in
	}
	in.State = st

	if cat, err := catalog.Load(); err == nil {
		in.SkillDrift = drift.Stale(cat, st)
	}
	if agents, err := catalog.LoadAgents(); err == nil {
		in.AgentDrift = drift.StaleAgents(agents, st)
	}
	if host != nil {
		in.EngramStale = drift.StaleEngram(host.MCPConfigPath, st)
	}
	return in
}

// doctorCommand runs the read-only ecosystem health check. handled is false when
// name is not "doctor", so main falls through to the configurator TUI. healthy is
// false when any check failed, which main turns into a non-zero exit code. With
// --repair, drift detected by the report is fixed by re-applying the managed
// catalog (a sync); other failures (missing prerequisites) are not auto-fixable.
//
//	capiko-ai doctor [--json] [--repair]
func doctorCommand(name string, args []string, out io.Writer) (handled, healthy bool, err error) {
	if name != "doctor" {
		return false, true, nil
	}
	asJSON, repair, err := parseDoctorArgs(args)
	if err != nil {
		return true, false, err
	}

	in := gatherDoctorInputs()
	r := doctor.Evaluate(in)

	// In JSON repair mode the repair outcome is the sole emitted object, so stdout
	// stays a single parseable document; suppress the report render in that case.
	if !(repair && asJSON) {
		if asJSON {
			s, err := doctor.RenderJSON(r)
			if err != nil {
				return true, false, err
			}
			fmt.Fprint(out, s)
		} else {
			fmt.Fprint(out, doctor.RenderText(r))
		}
	}

	if !repair {
		return true, r.Healthy(), nil
	}
	return repairDrift(out, asJSON, in, r)
}

// repairOutcome is the machine-readable result of a `doctor --repair` run.
type repairOutcome struct {
	DriftDetected bool   `json:"drift_detected"`
	Repaired      int    `json:"repaired"` // items re-applied by the sync
	Healthy       bool   `json:"healthy"`  // health after repair (non-drift failures persist)
	Message       string `json:"message"`
}

// repairDrift re-applies the managed catalog when `doctor --repair` finds drift.
// Drift is a warning (never a failure), so r.Healthy() already reflects the
// post-repair health: a missing prerequisite stays a failure that a sync cannot
// fix. The outcome is written as a text line after the report, or — in JSON mode
// — as the sole emitted object.
func repairDrift(out io.Writer, asJSON bool, in doctor.Inputs, r doctor.Report) (handled, healthy bool, err error) {
	hasDrift := len(in.SkillDrift) > 0 || len(in.AgentDrift) > 0 || in.EngramStale
	if !hasDrift {
		writeRepairOutcome(out, asJSON, repairOutcome{
			Healthy: r.Healthy(),
			Message: "nothing to repair: no drift detected",
		})
		return true, r.Healthy(), nil
	}

	sin, gerr := gatherSyncInputs()
	if gerr != nil {
		return true, false, gerr
	}
	if sin.hostExitCode != 0 {
		// Sync needs a Copilot host. The report already flags its absence as a
		// failure, so health is already false — surface the reason and stop.
		writeRepairOutcome(out, asJSON, repairOutcome{
			DriftDetected: true,
			Message:       "cannot repair: GitHub Copilot CLI not found",
		})
		return true, false, nil
	}

	n, serr := runSync(sin.host, sin.catalog, sin.agents, sin.store, sin.bkp)
	if serr != nil {
		writeRepairOutcome(out, asJSON, repairOutcome{
			DriftDetected: true,
			Message:       "repair failed: " + serr.Error(),
		})
		return true, false, nil
	}

	writeRepairOutcome(out, asJSON, repairOutcome{
		DriftDetected: true,
		Repaired:      n,
		Healthy:       r.Healthy(),
		Message:       fmt.Sprintf("repaired %d item(s): re-applied the managed catalog", n),
	})
	return true, r.Healthy(), nil
}

// writeRepairOutcome renders o as a single JSON object (JSON mode) or as a text
// line appended after the already-printed report. Write errors are ignored: out
// is typically os.Stdout, where a failure is unrecoverable and not worth raising.
func writeRepairOutcome(out io.Writer, asJSON bool, o repairOutcome) {
	if asJSON {
		b, err := json.MarshalIndent(o, "", "  ")
		if err != nil {
			return
		}
		fmt.Fprintln(out, string(b))
		return
	}
	fmt.Fprintf(out, "\nRepair: %s\n", o.Message)
}

// parseDoctorArgs parses the optional --json and --repair flags.
func parseDoctorArgs(args []string) (asJSON, repair bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--repair":
			repair = true
		default:
			return false, false, fmt.Errorf("doctor: unknown argument %q", a)
		}
	}
	return asJSON, repair, nil
}
