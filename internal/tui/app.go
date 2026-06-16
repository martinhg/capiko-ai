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

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/drift"
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
	appUpdatePrompt
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
	{"Configure SDD", "sdd", true},
	{"Configure engram", "engram", true},
	{"Upgrade tools", "upgrade", true},
	{"Upgrade + sync", "upgrade-sync", true},
	{"Install instructions", "instructions", true},
	{"Quit", "quit", true},
}

// App is the root Bubbletea model.
type App struct {
	catalog         []skill.Skill
	agentCatalog    []agent.Agent
	svc             services
	installed       map[string]bool
	installedAgents map[string]bool // agents present in AgentsDir
	state           appState
	err             error
	cursor          int
	active          screen
	latest          string   // newer version if an update is available; empty otherwise
	advisory        string   // remote advisory text; empty when absent or fetch failed
	stale           []string // installed skills whose catalog content has since changed
	staleAgents     []string // agents missing from state or whose checksum has changed
	menuTouched     bool     // true after any menu interaction or prompt dismissal
	restart         bool     // set after a successful self-update; main re-execs on exit
	postSync        bool     // set when the restart should sync skills with the new binary
}

// ShouldRestart reports whether a self-update succeeded and main should re-exec
// into the new binary after the program exits.
func (a App) ShouldRestart() bool { return a.restart }

// ShouldSyncAfterRestart reports whether the re-exec'd binary should sync skills
// on startup (the "Upgrade + sync" flow).
func (a App) ShouldSyncAfterRestart() bool { return a.postSync }

// NewApp builds the root model. The state and backup stores may be nil, in
// which case operations still work but are not recorded or snapshotted.
// agentCatalog may be nil; agents are simply skipped in that case.
func NewApp(catalog []skill.Skill, agentCatalog []agent.Agent, st *state.Store, bkp *backup.Store) App {
	return App{
		state:        appDetecting,
		catalog:      catalog,
		agentCatalog: agentCatalog,
		svc:          services{state: st, backup: bkp},
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(detectCmd, checkLatestCmd(a.svc.state), checkAdvisoryCmd())
}

type detectedMsg struct {
	host            *copilot.Host
	installed       map[string]bool
	installedAgents map[string]bool
	err             error
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
	if err != nil {
		return detectedMsg{host: h, err: err}
	}
	agentInst, err := h.InstalledAgents()
	return detectedMsg{host: h, installed: inst, installedAgents: agentInst, err: err}
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
			a.installedAgents = msg.installedAgents
			a.state = appMenu
			a.stale = staleSkills(a.svc.state, a.catalog)
			a.staleAgents = staleAgentsList(a.svc.state, a.agentCatalog)
		}
		return a, nil

	case advisoryMsg:
		a.advisory = msg.text
		return a, nil

	case latestVersionMsg:
		a.latest = msg.version
		if msg.version != "" && a.state == appMenu && !a.menuTouched {
			a.state = appUpdatePrompt
			a.cursor = promptDefaultCursor
		}
		return a, nil

	case restartMsg:
		a.restart, a.postSync = true, msg.sync
		return a, tea.Quit

	case backMsg:
		a.state, a.active = appMenu, nil
		return a, detectCmd // refresh the installed snapshot

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == appUpdatePrompt {
			return a.updatePrompt(msg)
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
	a.menuTouched = true
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
		a.active = newDetection(a.svc, a.catalog, a.installed)
	case it.id == "uninstall":
		a.active = newUninstall(a.svc, a.catalog, a.installed)
	case it.id == "sync":
		a.active = newSync(a.svc, a.catalog, a.agentCatalog)
	case it.id == "backups":
		a.active = newBackups(a.svc)
	case it.id == "upgrade":
		a.active = newUpgrade(a.svc, a.latest)
	case it.id == "sdd":
		a.active = newSDD(a.svc, a.catalog, a.installed, false)
	case it.id == "engram":
		a.active = newEngram(a.svc)
	case it.id == "upgrade-sync":
		a.active = newUpgradeSync(a.svc, a.catalog, a.latest)
	case it.id == "instructions":
		a.active = newInstructions(a.svc)
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
	case appUpdatePrompt:
		return a.viewPrompt()
	case appScreen:
		if a.active != nil {
			return head() + a.active.View()
		}
	}
	return head()
}

func (a App) viewMenu() string {
	var b strings.Builder

	b.WriteString(logo())
	b.WriteString("\n\n")
	b.WriteString(titleSty.Render("Capiko AI - v"+Version) + dimSty.Render("  ·  "+tagline))
	b.WriteString("\n")
	if banner := a.updateBanner(); banner != "" {
		b.WriteString(banner + "\n")
	}
	if banner := a.staleBanner(); banner != "" {
		b.WriteString(banner + "\n")
	}
	if a.advisory != "" {
		b.WriteString(dimSty.Render("Advisory: "+a.advisory) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(titleSty.Render("Menu"))
	b.WriteString("\n\n")

	for i, it := range menuItems {
		label := it.label
		// A star badges the upgrade item when a newer release is available.
		if it.id == "upgrade" && a.latest != "" {
			label += " ★"
		}
		if !it.ready {
			label += "  (coming soon)"
		}
		switch {
		case i == a.cursor:
			b.WriteString(titleSty.Render(menuCursor+label) + "\n")
		case !it.ready:
			b.WriteString(dimSty.Render("  "+label) + "\n")
		default:
			b.WriteString(textSty.Render("  "+label) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(dimSty.Render("↑/↓ move · enter select · q quit"))

	return menuBoxSty.Render(b.String()) + "\n"
}

// staleSkills loads the recorded state and reports which installed skills the
// current catalog would refresh. A nil store or a read error yields no drift —
// detection is best-effort and never blocks the menu.
func staleSkills(store *state.Store, catalog []skill.Skill) []string {
	if store == nil {
		return nil
	}
	st, err := store.Load()
	if err != nil {
		return nil
	}
	return drift.Stale(catalog, st)
}

// staleAgentsList loads the recorded state and reports which catalog agents are
// missing from state or have changed content. A nil store or read error yields
// no drift — detection is best-effort and never blocks the menu.
func staleAgentsList(store *state.Store, catalog []agent.Agent) []string {
	if store == nil {
		return nil
	}
	st, err := store.Load()
	if err != nil {
		return nil
	}
	return drift.StaleAgents(catalog, st)
}

// staleBanner renders the "out of date" line when the catalog has newer content
// than what is recorded in state, pointing the user at Sync configs. It covers
// both skills and agents so the user sees a single actionable notice.
func (a App) staleBanner() string {
	nSkills := len(a.stale)
	nAgents := len(a.staleAgents)
	total := nSkills + nAgents
	if total == 0 {
		return ""
	}

	var parts []string
	if nSkills > 0 {
		noun := "skill"
		if nSkills > 1 {
			noun = "skills"
		}
		parts = append(parts, fmt.Sprintf("%d %s", nSkills, noun))
	}
	if nAgents > 0 {
		noun := "agent"
		if nAgents > 1 {
			noun = "agents"
		}
		parts = append(parts, fmt.Sprintf("%d %s", nAgents, noun))
	}
	return warnSty.Render(strings.Join(parts, ", ") + " out of date · Sync configs to update")
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
