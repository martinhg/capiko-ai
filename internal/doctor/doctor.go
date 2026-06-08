// Package doctor composes capiko's existing detectors (sysinfo, copilot, state,
// drift) into a single read-only health report: one pass/warn/fail check per
// thing that can be wrong, each with a remedy. Evaluate is a pure function over
// already-gathered inputs so the diagnosis is fully table-testable; the cmd layer
// does the IO (detect, load, diff) and renders the result.
package doctor

import (
	"fmt"
	"strings"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/sysinfo"
)

// Status is the outcome of a single check.
type Status int

const (
	Pass Status = iota // healthy
	Warn               // works, but the user should act (drift, optional gap)
	Fail               // broken; capiko cannot do its job until fixed
)

func (s Status) String() string {
	switch s {
	case Pass:
		return "pass"
	case Warn:
		return "warn"
	case Fail:
		return "fail"
	default:
		return "unknown"
	}
}

// MarshalJSON emits the status as its string form ("pass"/"warn"/"fail") so JSON
// consumers read a label, not a positional int.
func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// Check is one diagnosis line.
type Check struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`           // what was observed
	Remedy string `json:"remedy,omitempty"` // how to fix it, when not Pass
}

// Report is the full set of checks.
type Report struct {
	Checks []Check `json:"checks"`
}

// Healthy reports whether no check failed. Warnings do not break health.
func (r Report) Healthy() bool {
	for _, c := range r.Checks {
		if c.Status == Fail {
			return false
		}
	}
	return true
}

// Counts returns how many checks landed in each status.
func (r Report) Counts() (pass, warn, fail int) {
	for _, c := range r.Checks {
		switch c.Status {
		case Pass:
			pass++
		case Warn:
			warn++
		case Fail:
			fail++
		}
	}
	return pass, warn, fail
}

// Inputs is everything Evaluate needs, gathered by the caller. Keeping it a plain
// struct (no IO) is what makes the diagnosis testable.
type Inputs struct {
	Env         sysinfo.Report // from sysinfo.Detect()
	CopilotHost *copilot.Host  // from copilot.Detect(); nil = not installed/initialized
	State       *state.State   // from state.Store.Load(); nil = no managed install yet
	StateErr    error          // non-nil = state file present but unreadable/corrupt
	SkillDrift  []string       // from drift.Stale(...)
	AgentDrift  []string       // from drift.StaleAgents(...)
	EngramStale bool           // from drift.StaleEngram(...): managed entry drifted or missing
}

// requiredDeps are the prerequisites capiko cannot work without; each gets its
// own check so a missing one names itself and its install hint.
var requiredDeps = []string{"copilot", "node", "npm", "pnpm", "git", "curl"}

// Evaluate runs every check against the gathered inputs and returns the report.
func Evaluate(in Inputs) Report {
	var r Report

	// Operating system support.
	if in.Env.Supported {
		r.add("Operating system", Pass, fmt.Sprintf("%s/%s supported", in.Env.OS, in.Env.Arch), "")
	} else {
		r.add("Operating system", Fail, fmt.Sprintf("%s/%s is not supported", in.Env.OS, in.Env.Arch),
			"capiko supports macOS, Linux, and Windows")
	}

	// Required prerequisites, one check each. "Copilot CLI" is named specially
	// since it is the host capiko configures.
	for _, name := range requiredDeps {
		dep, ok := findDep(in.Env, name)
		label := depLabel(name)
		switch {
		case !ok || !dep.Found:
			r.add(label, Fail, "not found on PATH", installHint(dep, name))
		default:
			r.add(label, Pass, "found "+versionOrPath(dep), "")
		}
	}

	// Copilot config: the binary can be installed but never initialized
	// (~/.copilot absent), which copilot.Detect signals with a nil host.
	if copilotFound(in.Env) {
		if in.CopilotHost != nil {
			r.add("Copilot config", Pass, "initialized at "+in.CopilotHost.ConfigDir, "")
		} else {
			r.add("Copilot config", Warn, "Copilot is installed but not initialized (~/.copilot missing)",
				"run `copilot` once and sign in, then re-run `capiko-ai doctor`")
		}
	}

	// State file.
	switch {
	case in.StateErr != nil:
		r.add("State file", Fail, "unreadable: "+in.StateErr.Error(),
			"inspect or remove ~/.capiko/state.json, then re-run capiko to rebuild it")
	case in.State == nil || in.State.Version == "":
		r.add("State file", Pass, "no managed install yet (nothing to verify)", "")
	default:
		r.add("State file", Pass, "valid (version "+in.State.Version+")", "")
	}

	// Drift: installed assets no longer match the embedded catalog. Only
	// meaningful against a managed baseline — without one, every catalog entry
	// looks "stale", so report n/a instead of crying wolf.
	managed := in.State != nil && in.State.Version != ""
	if managed {
		r.add("Skill drift", driftStatus(in.SkillDrift), driftDetail("skill", in.SkillDrift), driftRemedy(in.SkillDrift))
		r.add("Agent drift", driftStatus(in.AgentDrift), driftDetail("agent", in.AgentDrift), driftRemedy(in.AgentDrift))
	} else {
		r.add("Skill drift", Pass, "n/a (no managed install)", "")
		r.add("Agent drift", Pass, "n/a (no managed install)", "")
	}

	// Engram backend (optional). Only meaningful when the user has it managed.
	r.Checks = append(r.Checks, engramCheck(in))

	return r
}

func (r *Report) add(name string, status Status, detail, remedy string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail, Remedy: remedy})
}

func engramCheck(in Inputs) Check {
	managed := in.State != nil && in.State.Engram != nil && in.State.Engram.Enabled
	if !managed {
		return Check{Name: "Engram backend", Status: Pass, Detail: "not managed (optional)"}
	}
	dep, ok := findDep(in.Env, "engram")
	if !ok || !dep.Found {
		return Check{
			Name: "Engram backend", Status: Warn,
			Detail: "managed (mode " + in.State.Engram.ArtifactMode + ") but the engram binary is not on PATH",
			Remedy: "install engram from https://github.com/Gentleman-Programming/engram",
		}
	}
	if in.EngramStale {
		return Check{
			Name: "Engram backend", Status: Warn,
			Detail: "the engram MCP entry has drifted from the managed configuration",
			Remedy: "run Sync in capiko-ai to re-apply the engram wiring",
		}
	}
	return Check{Name: "Engram backend", Status: Pass, Detail: "configured (mode " + in.State.Engram.ArtifactMode + ")"}
}

func driftStatus(stale []string) Status {
	if len(stale) == 0 {
		return Pass
	}
	return Warn
}

func driftDetail(kind string, stale []string) string {
	if len(stale) == 0 {
		return "in sync with the embedded catalog"
	}
	return fmt.Sprintf("%d %s(s) differ from the catalog: %s", len(stale), kind, strings.Join(stale, ", "))
}

func driftRemedy(stale []string) string {
	if len(stale) == 0 {
		return ""
	}
	return "run Sync in capiko-ai to re-apply the managed assets"
}

func findDep(env sysinfo.Report, name string) (sysinfo.Dependency, bool) {
	for _, d := range env.Dependencies {
		if d.Name == name {
			return d, true
		}
	}
	return sysinfo.Dependency{}, false
}

func copilotFound(env sysinfo.Report) bool {
	d, ok := findDep(env, "copilot")
	return ok && d.Found
}

func depLabel(name string) string {
	if name == "copilot" {
		return "Copilot CLI"
	}
	return name
}

func versionOrPath(d sysinfo.Dependency) string {
	if d.Version != "" {
		return d.Name + " " + d.Version
	}
	return d.Name
}

func installHint(d sysinfo.Dependency, name string) string {
	if d.Install != "" {
		return d.Install
	}
	return "install " + name + " and ensure it is on your PATH"
}
