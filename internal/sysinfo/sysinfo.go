// Package sysinfo inspects the host environment so the configurator can show a
// "System Detection" summary before mounting the capiko layer: the OS/shell,
// whether the tools capiko relies on are present, the versions of its
// prerequisites, and which Copilot configs already exist.
package sysinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

// Tool is a command capiko looks for on the PATH (presence only).
type Tool struct {
	Name  string
	Found bool
	Path  string
}

// Dependency is a prerequisite reported with its detected version.
type Dependency struct {
	Name     string
	Required bool
	Found    bool
	Version  string // parsed from `<name> --version`, empty when not found
}

// Config records whether a Copilot config path capiko targets exists.
type Config struct {
	Name   string // display label
	Path   string
	Exists bool
}

// Report is the detected environment.
type Report struct {
	OS           string
	Arch         string
	Shell        string
	Supported    bool
	Tools        []Tool
	Dependencies []Dependency
	Configs      []Config
}

// Test seams.
var (
	lookPath    = exec.LookPath
	getenv      = os.Getenv
	userHomeDir = os.UserHomeDir
	runVersion  = func(name string, args ...string) (string, error) {
		out, err := exec.Command(name, args...).Output()
		return string(out), err
	}
)

// probedTools are the toolchain commands shown under "Tools" (presence only).
var probedTools = []string{"git", "curl", "brew", "node", "go"}

var versionRe = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// IsSupportedOS reports whether capiko supports the given GOOS.
func IsSupportedOS(goos string) bool {
	return goos == "darwin" || goos == "linux" || goos == "windows"
}

// Detect inspects the current environment. It reads the PATH, a couple of env
// vars, runs each prerequisite's `--version`, and stats the Copilot config paths.
func Detect() Report {
	r := Report{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Shell:     shell(),
		Supported: IsSupportedOS(runtime.GOOS),
	}
	for _, name := range probedTools {
		path, err := lookPath(name)
		r.Tools = append(r.Tools, Tool{Name: name, Found: err == nil, Path: path})
	}
	r.Dependencies = detectDependencies(runtime.GOOS)
	r.Configs = detectConfigs()
	return r
}

type depSpec struct {
	name     string
	required bool
	args     []string
}

// dependencySpecs is the capiko prerequisite list: the Copilot CLI it configures,
// the npm toolchain that installs Copilot, and git/curl. brew and go are optional
// install paths.
func dependencySpecs(goos string) []depSpec {
	specs := []depSpec{
		{"copilot", true, []string{"--version"}},
		{"node", true, []string{"--version"}},
		{"npm", true, []string{"--version"}},
		{"git", true, []string{"--version"}},
		{"curl", true, []string{"--version"}},
	}
	if goos == "darwin" {
		specs = append(specs, depSpec{"brew", false, []string{"--version"}})
	}
	return append(specs, depSpec{"go", false, []string{"version"}})
}

func detectDependencies(goos string) []Dependency {
	specs := dependencySpecs(goos)
	deps := make([]Dependency, 0, len(specs))
	for _, spec := range specs {
		d := Dependency{Name: spec.name, Required: spec.required}
		if out, err := runVersion(spec.name, spec.args...); err == nil {
			d.Found = true
			d.Version = versionRe.FindString(out)
		}
		deps = append(deps, d)
	}
	return deps
}

func detectConfigs() []Config {
	home, err := userHomeDir()
	if err != nil {
		return nil
	}
	cfg := filepath.Join(home, ".copilot")
	specs := []Config{
		{Name: "~/.copilot", Path: cfg},
		{Name: "~/.copilot/skills", Path: filepath.Join(cfg, "skills")},
		{Name: "settings.json", Path: filepath.Join(cfg, "settings.json")},
		{Name: "mcp-config.json", Path: filepath.Join(cfg, "mcp-config.json")},
	}
	for i := range specs {
		if _, err := os.Stat(specs[i].Path); err == nil {
			specs[i].Exists = true
		}
	}
	return specs
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
