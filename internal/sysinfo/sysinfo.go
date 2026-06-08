// Package sysinfo inspects the host environment so the configurator can show a
// "System Detection" summary before mounting the capiko layer: the OS/shell,
// whether the tools capiko relies on are present, the versions of its
// prerequisites, and which Copilot configs already exist.
package sysinfo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

// Tool is a command capiko looks for on the PATH (presence only).
type Tool struct {
	Name  string
	Found bool
	Path  string
}

// Dependency is a prerequisite reported with its detected version. When missing,
// Install carries the platform install command and Auto reports whether capiko
// can run it safely (no sudo / interactive prompt).
type Dependency struct {
	Name     string
	Required bool
	Found    bool
	Version  string // parsed from `<name> --version`, empty when not found
	Install  string // install command/hint, set when not found
	Auto     bool   // true when Install is safe to run via one-click
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
	runInstall  = func(cmd string) error { return exec.Command("sh", "-c", cmd).Run() }
	osReleaseFn = func() string {
		b, _ := os.ReadFile("/etc/os-release")
		return string(b)
	}
)

// Install runs a dependency's one-click install command. It refuses anything not
// marked Auto (sudo/manual installs), returning the manual hint instead.
func Install(d Dependency) error {
	if !d.Auto || d.Install == "" {
		return fmt.Errorf("install %s manually: %s", d.Name, d.Install)
	}
	return runInstall(d.Install)
}

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
// install paths; engram is the optional cross-session memory backend capiko can wire
// into Copilot (capiko works without it).
func dependencySpecs(goos string) []depSpec {
	specs := []depSpec{
		{"copilot", true, []string{"--version"}},
		{"node", true, []string{"--version"}},
		{"npm", true, []string{"--version"}},
		{"pnpm", true, []string{"--version"}},
		{"git", true, []string{"--version"}},
		{"curl", true, []string{"--version"}},
	}
	if goos == "darwin" {
		specs = append(specs, depSpec{"brew", false, []string{"--version"}})
	}
	specs = append(specs, depSpec{"go", false, []string{"version"}})
	return append(specs, depSpec{"engram", false, []string{"--version"}})
}

func detectDependencies(goos string) []Dependency {
	specs := dependencySpecs(goos)
	pm := packageManager(goos)
	deps := make([]Dependency, 0, len(specs))
	for _, spec := range specs {
		d := Dependency{Name: spec.name, Required: spec.required}
		if out, err := runVersion(spec.name, spec.args...); err == nil {
			d.Found = true
			d.Version = versionRe.FindString(out)
		} else {
			d.Install, d.Auto = installInfo(spec.name, pm)
		}
		deps = append(deps, d)
	}
	return deps
}

// Linux distro families capiko recognizes, mirroring gentle-ai's support matrix:
// Ubuntu/Debian (apt), Arch (pacman), and Fedora/RHEL (dnf).
const (
	linuxDistroUnknown = "unknown"
	linuxDistroUbuntu  = "ubuntu"
	linuxDistroDebian  = "debian"
	linuxDistroArch    = "arch"
	linuxDistroFedora  = "fedora"
)

// packageManager resolves the package manager capiko should reference for goos:
// brew on macOS (or Linux when Homebrew is installed), winget on Windows, and the
// distro-native manager (apt/pacman/dnf) on Linux. Returns "" when unsupported.
func packageManager(goos string) string {
	switch goos {
	case "darwin":
		return "brew"
	case "windows":
		return "winget"
	case "linux":
		if _, err := lookPath("brew"); err == nil {
			return "brew" // Linuxbrew, no sudo
		}
		switch detectLinuxDistro(osReleaseFn()) {
		case linuxDistroUbuntu, linuxDistroDebian:
			return "apt"
		case linuxDistroArch:
			return "pacman"
		case linuxDistroFedora:
			return "dnf"
		}
	}
	return ""
}

// detectLinuxDistro classifies /etc/os-release content into a known family using
// ID and ID_LIKE, so derivatives (Mint, Manjaro, Rocky, RHEL clones) map to their
// base distro's package manager.
func detectLinuxDistro(osRelease string) string {
	if strings.TrimSpace(osRelease) == "" {
		return linuxDistroUnknown
	}
	fields := map[string]string{}
	for _, line := range strings.Split(osRelease, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(parts[0]))
		val := strings.ToLower(strings.Trim(strings.TrimSpace(parts[1]), `"`))
		fields[key] = val
	}
	id, idLike := fields["ID"], fields["ID_LIKE"]
	switch {
	case matchesDistro(id, idLike, linuxDistroUbuntu, linuxDistroDebian):
		if id == linuxDistroDebian {
			return linuxDistroDebian
		}
		return linuxDistroUbuntu
	case matchesDistro(id, idLike, linuxDistroArch):
		return linuxDistroArch
	case matchesDistro(id, idLike, linuxDistroFedora, "rhel", "centos", "rocky", "almalinux", "nobara"):
		return linuxDistroFedora
	}
	return linuxDistroUnknown
}

// matchesDistro reports whether id, or any ID_LIKE token, is one of wants.
func matchesDistro(id, idLike string, wants ...string) bool {
	for _, w := range wants {
		if id == w {
			return true
		}
		for _, token := range strings.Fields(idLike) {
			if token == w {
				return true
			}
		}
	}
	return false
}

// installInfo returns how to install a missing dependency: the command (or a
// manual hint) and whether capiko may run it via one-click. Only no-sudo commands
// (Homebrew installs, the pnpm script) are auto-runnable; sudo, winget, and manual
// hints are reported with auto=false (shown, not run) — matching gentle-ai, which
// shows per-distro commands rather than auto-running them.
func installInfo(name, pm string) (cmd string, auto bool) {
	const brewScript = `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`
	const pnpmScript = "curl -fsSL https://get.pnpm.io/install.sh | sh -"

	switch name {
	case "pnpm":
		return pnpmScript, true // no sudo, safe to one-click
	case "brew":
		return brewScript, false // bootstrap prompts for sudo
	case "npm":
		return installInfo("node", pm) // npm ships with node
	case "engram":
		return manualHint("engram"), false // installed from its own release channel
	}

	switch pm {
	case "brew":
		return "brew install " + name, true
	case "winget":
		return wingetCmd(name), false
	case "apt":
		switch name {
		case "node":
			return "curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash - && sudo apt-get install -y nodejs", false
		case "go":
			return "sudo apt-get install -y golang", false
		default:
			return "sudo apt-get install -y " + name, false
		}
	case "pacman":
		switch name {
		case "node":
			return "sudo pacman -S --noconfirm nodejs npm", false
		default:
			return "sudo pacman -S --noconfirm " + name, false
		}
	case "dnf":
		switch name {
		case "node":
			return "curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash - && sudo dnf install -y nodejs", false
		case "go":
			return "sudo dnf install -y golang", false
		default:
			return "sudo dnf install -y " + name, false
		}
	default:
		return manualHint(name), false
	}
}

// wingetCmd returns the winget install command for a dependency on Windows.
func wingetCmd(name string) string {
	switch name {
	case "git":
		return "winget install --id Git.Git -e"
	case "node":
		return "winget install --id OpenJS.NodeJS.LTS -e"
	case "go":
		return "winget install --id GoLang.Go -e"
	default:
		return manualHint(name)
	}
}

// manualHint points at a tool's official install page when capiko has no command.
func manualHint(name string) string {
	switch name {
	case "git":
		return "install git from https://git-scm.com/"
	case "curl":
		return "install curl from https://curl.se/"
	case "node":
		return "install node from https://nodejs.org/"
	case "go":
		return "install go from https://go.dev/dl/"
	case "engram":
		return "install engram from https://github.com/Gentleman-Programming/engram"
	default:
		return "see the tool's website to install " + name
	}
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
		{Name: "~/.copilot/instructions", Path: filepath.Join(cfg, "instructions")},
		{Name: "settings.json", Path: filepath.Join(cfg, "settings.json")},
		{Name: "mcp-config.json", Path: filepath.Join(cfg, "mcp-config.json")},
	}
	// Extra instruction dirs Copilot loads via COPILOT_CUSTOM_INSTRUCTIONS_DIRS.
	// Surface them so capiko reflects every instruction source.
	for _, d := range copilot.CustomInstructionDirs() {
		specs = append(specs, Config{Name: d, Path: d})
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
