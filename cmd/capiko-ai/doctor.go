package main

import (
	"fmt"
	"io"
	"time"

	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/doctor"
	"github.com/martinhg/capiko-ai/internal/drift"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
	"github.com/martinhg/capiko-ai/internal/versions"
)

// gatherDoctorInputs collects the live environment for the doctor report. It is a
// package var so tests can stub it without touching the real PATH, home, or state.
var gatherDoctorInputs = func() doctor.Inputs {
	in := doctor.Inputs{Env: sysinfo.Detect(), Now: time.Now(), RecommendedEngram: versions.Engram}

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
// false when any check failed, which main turns into a non-zero exit code.
//
//	capiko-ai doctor [--json]
func doctorCommand(name string, args []string, out io.Writer) (handled, healthy bool, err error) {
	if name != "doctor" {
		return false, true, nil
	}
	asJSON, err := parseDoctorArgs(args)
	if err != nil {
		return true, false, err
	}

	r := doctor.Evaluate(gatherDoctorInputs())
	if asJSON {
		s, err := doctor.RenderJSON(r)
		if err != nil {
			return true, false, err
		}
		fmt.Fprint(out, s)
	} else {
		fmt.Fprint(out, doctor.RenderText(r))
	}
	return true, r.Healthy(), nil
}

// parseDoctorArgs parses the optional --json flag.
func parseDoctorArgs(args []string) (asJSON bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		default:
			return false, fmt.Errorf("doctor: unknown argument %q", a)
		}
	}
	return asJSON, nil
}
