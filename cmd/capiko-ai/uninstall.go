package main

import (
	"fmt"
	"io"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/headless"
	"github.com/martinhg/capiko-ai/internal/state"
	"github.com/martinhg/capiko-ai/internal/tui"
)

// uninstallInputs bundles everything uninstallCommand needs from the outside
// world, gathered by gatherUninstallInputs so tests can stub the environment.
type uninstallInputs struct {
	host         *copilot.Host
	hostExitCode int   // 0 success, 1 detect error, 2 Copilot not found
	hostErr      error // non-nil only when hostExitCode == 1
	store        *state.Store
	bkp          *backup.Store
}

// gatherUninstallInputs collects the live environment for the uninstall
// command. It is a package var so tests can stub it without touching the real
// PATH, home directory, or filesystem. Unlike install/sync, no catalog is
// loaded: uninstall discovers what to remove from state (or disk when store is
// nil), so catalog knowledge is not needed.
var gatherUninstallInputs = func() (uninstallInputs, error) {
	host, exitCode := requireHost(copilot.Detect)
	in := uninstallInputs{host: host, hostExitCode: exitCode}
	if exitCode != 0 {
		if exitCode == 1 {
			// requireHost swallows the detect error; call again to surface it for
			// exit reporting. Cheap: only on the already-failing path.
			_, err := copilot.Detect()
			in.hostErr = err
		}
		return in, nil // early return; uninstallCommand handles the exit code
	}

	store, _ := state.DefaultStore() // nil-tolerant
	in.store = store
	bkp, _ := backup.DefaultStore() // nil-tolerant
	in.bkp = bkp
	return in, nil
}

// uninstallCommand removes every managed skill and agent from the Copilot
// host. handled is false when name is not "uninstall", so main falls through
// to the configurator TUI.
//
//	capiko-ai uninstall [--json] [--all]
func uninstallCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "uninstall" {
		return false, 0, nil
	}
	asJSON, _, err := parseUninstallArgs(args)
	if err != nil {
		return true, 1, err
	}

	in, err := gatherUninstallInputs()
	if err != nil {
		return true, 1, err
	}

	if in.hostExitCode != 0 {
		msg := "GitHub Copilot CLI not found"
		if in.hostExitCode == 1 && in.hostErr != nil {
			msg = in.hostErr.Error()
		}
		r := headless.FromReconcileResult("uninstall", tui.ReconcileResult{}, fmt.Errorf("%s", msg))
		renderUninstall(out, r, asJSON)
		return true, in.hostExitCode, in.hostErr
	}

	result, err := tui.UninstallAll(in.host, in.store, in.bkp)
	if err != nil {
		r := headless.FromReconcileResult("uninstall", tui.ReconcileResult{}, err)
		renderUninstall(out, r, asJSON)
		return true, 1, nil
	}

	r := headless.FromReconcileResult("uninstall", result, nil)
	renderUninstall(out, r, asJSON)
	return true, 0, nil
}

// renderUninstall writes r to out as JSON or text, ignoring render errors past
// the point of no return (out is typically os.Stdout, where a write failure is
// unrecoverable and not worth surfacing as a command failure).
func renderUninstall(out io.Writer, r headless.CommandResult, asJSON bool) {
	if asJSON {
		_ = headless.RenderJSON(out, r)
		return
	}
	headless.RenderText(out, r)
}

// parseUninstallArgs parses the optional --json and --all flags. --all is
// accepted for explicitness but is the only mode uninstall supports.
func parseUninstallArgs(args []string) (asJSON, all bool, err error) {
	for _, a := range args {
		switch a {
		case "--json":
			asJSON = true
		case "--all":
			all = true
		default:
			return false, false, fmt.Errorf("uninstall: unknown argument %q", a)
		}
	}
	return asJSON, all, nil
}
