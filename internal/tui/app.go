// Package tui renders the capiko-ai configurator with Bubbletea.
//
// App is the root model: it detects the Copilot host, shows the main menu, and
// routes to the active screen. Each menu option becomes a screen that renders
// its own body and returns to the menu via backMsg. Screens that share a flow
// (install, uninstall) reuse the same reconcile engine in selector.go.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// services bundles the cross-cutting dependencies the screens need, so adding a
// new one does not balloon every constructor's signature. host is filled in
// after detection; state and backup may be nil (operations still work, just
// without recording or snapshots).
type services struct {
	host   *copilot.Host
	state  *state.Store
	backup *backup.Store
}

const tagline = "Ecosystem, Frameworks, Workflows"

// screen is one full-screen view routed by the App. Screens render their own
// body (not the capiko banner, which the App draws) and signal a return to the
// menu by emitting backMsg.
type screen interface {
	Update(tea.Msg) (screen, tea.Cmd)
	View() string
}

// back is a command that returns to the main menu.
func back() tea.Msg { return backMsg{} }

type backMsg struct{}

type appState int

const (
	appDetecting appState = iota
	appNotFound
	appMenu
	appScreen
	appFailed
)

type menuItem struct {
	label string
	id    string
	ready bool
}

var menuItems = []menuItem{
	{"Start installation", "install", true},
	{"Managed uninstall", "uninstall", true},
	{"Sync configs", "sync", true},
	{"Manage backups", "backups", true},
	{"Upgrade tools", "upgrade", true},
	{"Upgrade + sync", "upgrade-sync", false},
	{"Quit", "quit", true},
}

// App is the root Bubbletea model.
type App struct {
	catalog   []skill.Skill
	svc       services
	installed map[string]bool
	state     appState
	err       error
	cursor    int
	active    screen
	latest    string // newer version if an update is available; empty otherwise
	restart   bool   // set after a successful self-update; main re-execs on exit
}

// ShouldRestart reports whether a self-update succeeded and main should re-exec
// into the new binary after the program exits.
func (a App) ShouldRestart() bool { return a.restart }

// NewApp builds the root model. The state and backup stores may be nil, in
// which case operations still work but are not recorded or snapshotted.
func NewApp(catalog []skill.Skill, st *state.Store, bkp *backup.Store) App {
	return App{
		state:   appDetecting,
		catalog: catalog,
		svc:     services{state: st, backup: bkp},
	}
}

func (a App) Init() tea.Cmd { return tea.Batch(detectCmd, checkLatestCmd) }

type detectedMsg struct {
	host      *copilot.Host
	installed map[string]bool
	err       error
}

func detectCmd() tea.Msg {
	h, err := copilot.Detect()
	if err != nil {
		return detectedMsg{err: err}
	}
	if h == nil {
		return detectedMsg{}
	}
	inst, err := h.InstalledSkills()
	return detectedMsg{host: h, installed: inst, err: err}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case detectedMsg:
		switch {
		case msg.err != nil:
			a.state, a.err = appFailed, msg.err
		case msg.host == nil:
			a.state = appNotFound
		default:
			a.svc.host, a.installed = msg.host, msg.installed
			a.state = appMenu
		}
		return a, nil

	case latestVersionMsg:
		a.latest = msg.version
		return a, nil

	case restartMsg:
		a.restart = true
		return a, tea.Quit

	case backMsg:
		a.state, a.active = appMenu, nil
		return a, detectCmd // refresh the installed snapshot

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == appMenu {
			return a.updateMenu(msg)
		}
	}

	if a.state == appScreen && a.active != nil {
		next, cmd := a.active.Update(msg)
		a.active = next
		return a, cmd
	}
	return a, nil
}

func (a App) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return a, tea.Quit
	case "up", "k":
		if a.cursor > 0 {
			a.cursor--
		}
	case "down", "j":
		if a.cursor < len(menuItems)-1 {
			a.cursor++
		}
	case "enter":
		return a.open(menuItems[a.cursor])
	}
	return a, nil
}

func (a App) open(it menuItem) (tea.Model, tea.Cmd) {
	switch {
	case it.id == "quit":
		return a, tea.Quit
	case !it.ready:
		a.active = newSoon(it.label)
	case it.id == "install":
		a.active = newInstall(a.svc, a.catalog, a.installed)
	case it.id == "uninstall":
		a.active = newUninstall(a.svc, a.catalog, a.installed)
	case it.id == "sync":
		a.active = newSync(a.svc, a.catalog)
	case it.id == "backups":
		a.active = newBackups(a.svc)
	case it.id == "upgrade":
		a.active = newUpgrade(a.latest)
	default:
		a.active = newSoon(it.label)
	}
	a.state = appScreen
	return a, nil
}

func (a App) View() string {
	switch a.state {
	case appDetecting:
		return head() + "Detecting Copilot CLI…\n"
	case appNotFound:
		return head() + warnSty.Render("Copilot CLI not detected.") + "\n" +
			dimSty.Render("Install it and run `copilot` once to log in, then retry.") + "\n\n" +
			dimSty.Render("q to quit") + "\n"
	case appFailed:
		return head() + errSty.Render("Error: "+a.err.Error()) + "\n\n" +
			dimSty.Render("q to quit") + "\n"
	case appMenu:
		return a.viewMenu()
	case appScreen:
		if a.active != nil {
			return head() + a.active.View()
		}
	}
	return head()
}

func (a App) viewMenu() string {
	var items strings.Builder
	for i, it := range menuItems {
		label := it.label
		if !it.ready {
			label += "  (coming soon)"
		}
		switch {
		case i == a.cursor:
			items.WriteString(titleSty.Render("› "+label) + "\n")
		case !it.ready:
			items.WriteString(dimSty.Render("  "+label) + "\n")
		default:
			items.WriteString(textSty.Render("  "+label) + "\n")
		}
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(brandColor).
		Padding(0, 2).
		Render(strings.TrimRight(items.String(), "\n"))

	header := titleSty.Render("CAPIKO-AI "+Version) + dimSty.Render("  ·  "+tagline)

	parts := []string{logo(), "", header}
	if banner := a.updateBanner(); banner != "" {
		parts = append(parts, banner)
	}
	parts = append(parts,
		"",
		titleSty.Render("Menu"),
		box,
		"",
		dimSty.Render("↑/↓ move · enter select · q quit"),
	)

	return lipgloss.JoinVertical(lipgloss.Left, parts...) + "\n"
}

// updateBanner renders the "update available" line when a newer version is
// known. The UI is ready; wiring a real version check (GitHub releases / brew)
// is a future step, so latest is empty by default and the banner stays hidden.
func (a App) updateBanner() string {
	if a.latest == "" || a.latest == Version {
		return ""
	}
	return warnSty.Render(fmt.Sprintf("Update available: capiko-ai %s → %s", Version, a.latest))
}
