// Package sysinfo inspects the host environment so the configurator can show a
// "System Detection" summary before mounting the capiko layer: the OS/shell and
// whether the tools capiko relies on (the Copilot CLI and its toolchain) are
// present.
package sysinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Tool is a command capiko looks for on the PATH.
type Tool struct {
	Name  string
	Found bool
	Path  string // absolute path when found
}

// Report is the detected environment.
type Report struct {
	OS    string
	Arch  string
	Shell string
	Tools []Tool
}

// Test seams.
var (
	lookPath = exec.LookPath
	getenv   = os.Getenv
)

// probed lists the commands capiko cares about, in display order: the Copilot
// CLI it configures, the npm toolchain that installs Copilot, and git.
var probed = []string{"copilot", "node", "npm", "git"}

// Detect inspects the current environment. It only reads the PATH and a couple
// of env vars, so it is fast and side-effect free.
func Detect() Report {
	r := Report{
		OS:    runtime.GOOS,
		Arch:  runtime.GOARCH,
		Shell: shell(),
	}
	for _, name := range probed {
		path, err := lookPath(name)
		r.Tools = append(r.Tools, Tool{Name: name, Found: err == nil, Path: path})
	}
	return r
}

// shell returns the base name of the user's shell, or "unknown".
func shell() string {
	if s := getenv("SHELL"); s != "" {
		return filepath.Base(s)
	}
	if s := getenv("COMSPEC"); s != "" { // Windows
		return filepath.Base(s)
	}
	return "unknown"
}
