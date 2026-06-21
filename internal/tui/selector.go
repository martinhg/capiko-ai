package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// selector is the declarative reconcile screen shared by install and uninstall.
// Each entry's checkbox is the DESIRED installed state, seeded from disk;
// applying installs what was marked and removes what was unmarked.
type selector struct {
	svc         services
	title       string
	emptyMsg    string
	items       []skill.Skill
	installed   map[string]bool
	desired     map[int]bool
	cursor      int
	state       selState
	result      reconcileResult
	err         error
	resolveDeps bool // install flow: pull in transitive dependencies of marked skills
}

type selState int

const (
	selPicking selState = iota
	selApplying
	selDone
	selFailed
)

type reconcileResult struct {
	installed []string
	removed   []string
}

func (r reconcileResult) empty() bool {
	return len(r.installed) == 0 && len(r.removed) == 0
}

type reconciledMsg struct {
	result reconcileResult
	err    error
}

// newInstall offers the full catalog, seeded with what is already on disk.
func newInstall(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	desired := make(map[int]bool, len(catalog))
	for i, s := range catalog {
		desired[i] = installed[s.Name]
	}
	return &selector{
		svc:         svc,
		title:       "Start installation",
		items:       catalog,
		installed:   installed,
		desired:     desired,
		resolveDeps: true,
	}
}

// newUninstall lists only installed catalog skills, all marked; unmark to remove.
func newUninstall(svc services, catalog []skill.Skill, installed map[string]bool) screen {
	var items []skill.Skill
	for _, s := range catalog {
		if installed[s.Name] {
			items = append(items, s)
		}
	}
	desired := make(map[int]bool, len(items))
	for i := range items {
		desired[i] = true
	}
	return &selector{
		svc:       svc,
		title:     "Managed uninstall",
		emptyMsg:  "No capiko skills installed.",
		items:     items,
		installed: installed,
		desired:   desired,
	}
}

func (s *selector) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case reconciledMsg:
		if msg.err != nil {
			s.state, s.err = selFailed, msg.err
			return s, nil
		}
		s.state, s.result = selDone, msg.result
		return s, nil
	case tea.KeyMsg:
		return s.handleKey(msg)
	}
	return s, nil
}

func (s *selector) handleKey(msg tea.KeyMsg) (screen, tea.Cmd) {
	if k := msg.String(); k == "q" || k == "esc" {
		return s, back
	}
	if s.state == selDone || s.state == selFailed {
		return s, back // any key returns to the menu
	}
	if s.state != selPicking || len(s.items) == 0 {
		return s, nil
	}
	switch msg.String() {
	case "up", "k":
		if s.cursor > 0 {
			s.cursor--
		}
	case "down", "j":
		if s.cursor < len(s.items)-1 {
			s.cursor++
		}
	case " ":
		s.desired[s.cursor] = !s.desired[s.cursor]
	case "a":
		s.toggleAll()
	case "enter":
		if s.hasChanges() {
			return newReview(s), nil // confirm before applying
		}
	}
	return s, nil
}

// requiredNames is the single source of truth for what the current selection
// should result in being installed: every marked skill plus, for the install
// flow, its transitive dependencies. If resolution fails it falls back to the
// raw marked set so a graph error never blocks the user mid-flow (the catalog is
// already validated at load time, and doctor reports broken chains).
func (s *selector) requiredNames() map[string]bool {
	marked := make(map[string]bool)
	var names []string
	for i, sk := range s.items {
		if s.desired[i] {
			marked[sk.Name] = true
			names = append(names, sk.Name)
		}
	}
	if !s.resolveDeps {
		return marked
	}
	resolved, err := skill.ResolveDependencies(s.items, names)
	if err != nil {
		return marked
	}
	want := make(map[string]bool, len(resolved))
	for _, n := range resolved {
		want[n] = true
	}
	return want
}

// plan returns the skills the current selection would install and remove.
func (s *selector) plan() (install, remove []string) {
	want := s.requiredNames()
	for _, sk := range s.items {
		switch w, have := want[sk.Name], s.installed[sk.Name]; {
		case w && !have:
			install = append(install, sk.Name)
		case !w && have:
			remove = append(remove, sk.Name)
		}
	}
	return install, remove
}

func (s *selector) toggleAll() {
	all := true
	for i := range s.items {
		if !s.desired[i] {
			all = false
			break
		}
	}
	for i := range s.items {
		s.desired[i] = !all
	}
}

func (s *selector) hasChanges() bool {
	want := s.requiredNames()
	for _, sk := range s.items {
		if want[sk.Name] != s.installed[sk.Name] {
			return true
		}
	}
	return false
}

func (s *selector) reconcileCmd() tea.Cmd {
	host, store, bkp := s.svc.host, s.svc.state, s.svc.backup
	type op struct {
		sk      skill.Skill
		install bool
	}
	var ops []op
	var affected []string
	want := s.requiredNames()
	for _, sk := range s.items {
		w, have := want[sk.Name], s.installed[sk.Name]
		switch {
		case w && !have:
			ops = append(ops, op{sk, true})
			affected = append(affected, sk.Name)
		case !w && have:
			ops = append(ops, op{sk, false})
			affected = append(affected, sk.Name)
		}
	}
	return func() tea.Msg {
		// Safety first: snapshot affected skills before touching them. If the
		// backup fails, abort without mutating.
		if bkp != nil && len(affected) > 0 {
			if _, err := bkp.Create(host.SkillsDir, "reconcile", Version, affected); err != nil {
				return reconciledMsg{err: fmt.Errorf("backup failed, aborting: %w", err)}
			}
		}
		var res reconcileResult
		var recorded []state.Installed
		for _, o := range ops {
			if o.install {
				if _, err := o.sk.Install(host.SkillsDir); err != nil {
					return reconciledMsg{err: fmt.Errorf("installing %s: %w", o.sk.Name, err)}
				}
				res.installed = append(res.installed, o.sk.Name)
				recorded = append(recorded, state.Installed{Name: o.sk.Name, Checksum: state.Checksum(o.sk.CanonicalContent())})
				continue
			}
			if err := host.UninstallSkill(o.sk.Name); err != nil {
				return reconciledMsg{err: fmt.Errorf("removing %s: %w", o.sk.Name, err)}
			}
			res.removed = append(res.removed, o.sk.Name)
		}
		if store != nil {
			if err := store.Apply(Version, recorded, res.removed); err != nil {
				return reconciledMsg{err: fmt.Errorf("recording state: %w", err)}
			}
		}
		return reconciledMsg{result: res}
	}
}

func (s *selector) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render(s.title) + "\n\n")

	switch s.state {
	case selApplying:
		b.WriteString("Applying changes…\n")
		return b.String()
	case selDone:
		if s.result.empty() {
			b.WriteString(dimSty.Render("No changes.") + "\n")
		}
		for _, n := range s.result.installed {
			b.WriteString(okSty.Render("  + ") + n + "\n")
		}
		for _, n := range s.result.removed {
			b.WriteString(warnSty.Render("  - ") + n + "\n")
		}
		b.WriteString("\n" + dimSty.Render("any key to go back") + "\n")
		return b.String()
	case selFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	if len(s.items) == 0 {
		b.WriteString(dimSty.Render(s.emptyMsg) + "\n\n")
		b.WriteString(dimSty.Render("q to go back") + "\n")
		return b.String()
	}

	for i, sk := range s.items {
		cursor := "  "
		if i == s.cursor {
			cursor = titleSty.Render(menuCursor)
		}
		box := "[ ]"
		if s.desired[i] {
			box = okSty.Render("[x]")
		}
		var tag string
		switch have := s.installed[sk.Name]; {
		case s.desired[i] && !have:
			tag = okSty.Render("  + install")
		case !s.desired[i] && have:
			tag = warnSty.Render("  - remove")
		case have:
			tag = dimSty.Render("  (installed)")
		}
		fmt.Fprintf(&b, "%s%s %s%s\n", cursor, box, sk.Name, tag)
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · space toggle · a all · enter apply · q back") + "\n")
	return b.String()
}
