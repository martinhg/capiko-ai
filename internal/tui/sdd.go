package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/sdd"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// applySDD injects the SDD orchestrator block (with the given per-phase model
// and effort assignments) into Copilot's instructions file, backing the file up
// only when it changes, then records the assignments in state. Shared by the
// config screen and the post-sync re-apply.
func applySDD(host *copilot.Host, store *state.Store, bkp *backup.Store, models, efforts map[string]string, strict bool) error {
	if host == nil {
		return nil
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, sdd.MarkerStart, sdd.MarkerEnd, sdd.Render(models, efforts, strict))
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("sdd", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	if store != nil {
		if err := store.SetSDDModels(models); err != nil {
			return err
		}
		if err := store.SetSDDEfforts(efforts); err != nil {
			return err
		}
		return store.SetStrictTDD(strict)
	}
	return nil
}

// sddScreen configures the model and reasoning effort assigned to each SDD
// phase. Each phase cycles model (←/→) or effort (e), or takes a custom model
// id (c). Apply injects the orchestrator block; in the install flow it then
// continues to the skill selector.
type sddScreen struct {
	svc       services
	catalog   []skill.Skill
	installed map[string]bool
	inFlow    bool // reached from install flow → Apply continues to the selector
	models    map[string]string
	efforts   map[string]string
	strict    bool // strict TDD for apply/verify
	cursor    int  // 0..len-1 phases, len = Apply, len+1 = Back
	editing   bool
	editBuf   string
	state     sddState
	err       error
}

type sddState int

const (
	sddPicking sddState = iota
	sddApplying
	sddFailed
)

type sddAppliedMsg struct{ err error }

func newSDD(svc services, catalog []skill.Skill, installed map[string]bool, inFlow bool) screen {
	models := sdd.DefaultAssignments()
	efforts := sdd.DefaultEfforts()
	strict := false
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil {
			for k, v := range st.SDDModels {
				if _, ok := models[k]; ok {
					models[k] = v
				}
			}
			for k, v := range st.SDDEfforts {
				if _, ok := efforts[k]; ok {
					efforts[k] = v
				}
			}
			strict = st.StrictTDD
		}
	}
	return &sddScreen{svc: svc, catalog: catalog, installed: installed, inFlow: inFlow, models: models, efforts: efforts, strict: strict}
}

func (s *sddScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case sddAppliedMsg:
		if msg.err != nil {
			s.state, s.err = sddFailed, msg.err
			return s, nil
		}
		if s.inFlow {
			return newInstall(s.svc, s.catalog, s.installed), nil
		}
		return s, back
	case tea.KeyMsg:
		if s.editing {
			return s.handleEdit(msg)
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		}
		if s.state == sddFailed {
			return s, back
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(sdd.Phases)+1 {
				s.cursor++
			}
		case "left", "h":
			s.cycle(-1)
		case "right", "l":
			s.cycle(1)
		case "e":
			s.cycleEffort(1)
		case "t":
			s.strict = !s.strict
		case "c":
			if s.onPhase() {
				s.editing, s.editBuf = true, ""
			}
		case "enter":
			switch s.cursor {
			case len(sdd.Phases): // Apply
				s.state = sddApplying
				return s, s.applyCmd()
			case len(sdd.Phases) + 1: // Back
				return s, back
			}
		}
	}
	return s, nil
}

func (s *sddScreen) handleEdit(msg tea.KeyMsg) (screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		v := strings.TrimSpace(s.editBuf)
		if v == "" {
			v = sdd.DefaultModel
		}
		s.models[sdd.Phases[s.cursor]] = v
		s.editing = false
	case tea.KeyEsc:
		s.editing = false
	case tea.KeyBackspace:
		if len(s.editBuf) > 0 {
			s.editBuf = s.editBuf[:len(s.editBuf)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		s.editBuf += string(msg.Runes)
	}
	return s, nil
}

func (s *sddScreen) onPhase() bool { return s.cursor < len(sdd.Phases) }

func (s *sddScreen) cycle(delta int) {
	if !s.onPhase() {
		return
	}
	phase := sdd.Phases[s.cursor]
	idx := -1
	for i, m := range sdd.Models {
		if m == s.models[phase] {
			idx = i
			break
		}
	}
	n := len(sdd.Models)
	if idx == -1 { // currently a custom value
		if delta > 0 {
			idx = -1
		} else {
			idx = n
		}
	}
	s.models[phase] = sdd.Models[((idx+delta)%n+n)%n]
}

func (s *sddScreen) cycleEffort(delta int) {
	if !s.onPhase() {
		return
	}
	phase := sdd.Phases[s.cursor]
	idx := 0
	for i, e := range sdd.Efforts {
		if e == s.efforts[phase] {
			idx = i
			break
		}
	}
	n := len(sdd.Efforts)
	s.efforts[phase] = sdd.Efforts[((idx+delta)%n+n)%n]
}

func (s *sddScreen) applyCmd() tea.Cmd {
	host, store, bkp, models, efforts, strict := s.svc.host, s.svc.state, s.svc.backup, s.models, s.efforts, s.strict
	return func() tea.Msg {
		if err := applySDD(host, store, bkp, models, efforts, strict); err != nil {
			return sddAppliedMsg{err: err}
		}
		return sddAppliedMsg{err: applyTriggerRules(host, store, bkp, true)}
	}
}

func (s *sddScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure SDD models") + "\n\n")
	b.WriteString(dimSty.Render("Run the orchestrator on the top model; cheaper phases auto-downgrade.") + "\n")
	strictVal := dimSty.Render("off")
	if s.strict {
		strictVal = okSty.Render("on")
	}
	b.WriteString("  " + titleSty.Render(pad("Strict TDD", 13)) + "  " + strictVal + dimSty.Render("  (t to toggle)") + "\n\n")

	switch s.state {
	case sddApplying:
		b.WriteString("Applying SDD orchestrator…\n")
		return b.String()
	case sddFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	for i, phase := range sdd.Phases {
		marker := "  "
		nameSty := textSty
		if i == s.cursor {
			marker = titleSty.Render(menuCursor)
			nameSty = titleSty
		}
		model := s.models[phase]
		if s.editing && i == s.cursor {
			model = s.editBuf + "▏"
		}
		effort := s.efforts[phase]
		fmt.Fprintf(&b, "%s%s  %s  %s\n", marker, nameSty.Render(pad(phase, 13)), dimSty.Render(pad(model, 20)), dimSty.Render(effort))
	}
	b.WriteString("\n")

	for i, opt := range []string{"Apply", "Back"} {
		marker := "  "
		optSty := textSty
		if s.cursor == len(sdd.Phases)+i {
			marker = titleSty.Render(menuCursor)
			optSty = titleSty
		}
		b.WriteString(marker + optSty.Render(opt) + "\n")
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · ←/→ model · e effort · c custom · t strict TDD · enter select · esc back") + "\n")
	return b.String()
}
