package main

import (
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/catalog"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/headless"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

// installInputs bundles everything installCommand needs from the outside
// world, gathered by gatherInstallInputs so tests can stub the environment.
type installInputs struct {
	host         *copilot.Host
	hostExitCode int   // 0 success, 1 detect error, 2 Copilot not found
	hostErr      error // non-nil only when hostExitCode == 1
	catalog      []skill.Skill
	agents       []agent.Agent
	store        *state.Store
	bkp          *backup.Store
}

// gatherInstallInputs collects the live environment for the install command.
// It is a package var so tests can stub it without touching the real PATH,
// home directory, or embedded catalog.
var gatherInstallInputs = func() (installInputs, error) {
	host, exitCode := requireHost(copilot.Detect)
	in := installInputs{host: host, hostExitCode: exitCode}
	if exitCode != 0 {
		if exitCode == 1 {
			// requireHost does not propagate the underlying error, so detect
			// again to surface it for exit reporting. This second call is
			// cheap (no I/O beyond what Detect already does) and only runs on
			// the already-erroring path.
			_, err := copilot.Detect()
			in.hostErr = err
		}
		return in, nil // early return; installCommand handles the exit code
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

// installCommand installs every catalog skill and agent that is not already
// present (additive-only). handled is false when name is not "install", so
// main falls through to the configurator TUI.
//
//	capiko-ai install [--json] [--all]
func installCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "install" {
		return false, 0, nil
	}
	asJSON, _, err := parseInstallArgs(args)
	if err != nil {
		return true, 1, err
	}

	in, err := gatherInstallInputs()
	if err != nil {
		return true, 1, err
	}

	if in.hostExitCode != 0 {
		msg := "GitHub Copilot CLI not found"
		if in.hostExitCode == 1 && in.hostErr != nil {
			msg = in.hostErr.Error()
		}
		r := headless.FromReconcileResult("install", tui.ReconcileResult{}, fmt.Errorf("%s", msg))
		renderInstall(out, r, asJSON)
		return true, in.hostExitCode, in.hostErr
	}

	result, err := tui.InstallAll(in.host, in.catalog, in.agents, in.store, in.bkp)
	if err != nil {
		r := headless.FromReconcileResult("install", tui.ReconcileResult{}, err)
		renderInstall(out, r, asJSON)
		return true, 1, nil
	}

	r := headless.FromReconcileResult("install", result, nil)
	renderInstall(out, r, asJSON)
	return true, 0, nil
}

// renderInstall writes r to out as JSON or text, ignoring render errors past
// the point of no return (out is typically os.Stdout, where a write failure
// is unrecoverable and not worth surfacing as a command failure).
func renderInstall(out io.Writer, r headless.CommandResult, asJSON bool) {
	if asJSON {
		_ = headless.RenderJSON(out, r)
		return
	}
	headless.RenderText(out, r)
}

// parseInstallArgs parses the optional --json and --all flags. --all is
// accepted for explicitness but is the only mode install supports.
func parseInstallArgs(args []string) (asJSON, all bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--all":
			all = true
		default:
			return false, false, fmt.Errorf("install: unknown argument %q", a)
		}
	}
	return asJSON, all, nil
}
