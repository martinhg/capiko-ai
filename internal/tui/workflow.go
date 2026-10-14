package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/odd"
	"github.com/martinhg/capiko-ai/internal/scoped"
	"github.com/martinhg/capiko-ai/internal/sdd"
)

type workflowChoice int

const (
	workflowODD workflowChoice = iota
	workflowSDD
)

var workflowChoices = []struct {
	id   workflowChoice
	name string
	desc string
}{
	{workflowODD, "ODD", "Organic Driven Development (default)"},
	{workflowSDD, "SDD (legacy)", "Spec-Driven Development orchestrator"},
}

type workflowState int

const (
	wfChoosing workflowState = iota
	wfApplying
	wfDone
	wfFailed
)

type workflowScreen struct {
	svc     services
	cursor  int
	current workflowChoice
	state   workflowState
	err     error
}

type workflowAppliedMsg struct{ err error }

func newWorkflow(svc services) screen {
	cur := workflowODD
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.IsSDDWorkflow() {
			cur = workflowSDD
		}
	}
	return &workflowScreen{svc: svc, current: cur, cursor: int(cur)}
}

func (w *workflowScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case workflowAppliedMsg:
		if msg.err != nil {
			w.state, w.err = wfFailed, msg.err
			return w, nil
		}
		w.state = wfDone
		w.current = workflowChoice(w.cursor)
		return w, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc":
			return w, back
		case "up", "k":
			if w.cursor > 0 {
				w.cursor--
			}
		case "down", "j":
			if w.cursor < len(workflowChoices)-1 {
				w.cursor++
			}
		case "enter":
			if w.state == wfDone || w.state == wfFailed {
				return w, back
			}
			chosen := workflowChoice(w.cursor)
			if chosen == w.current {
				return w, back
			}
			w.state = wfApplying
			return w, w.applyCmd(chosen)
		}
	}
	return w, nil
}

func (w *workflowScreen) applyCmd(choice workflowChoice) tea.Cmd {
	svc := w.svc
	return func() tea.Msg {
		var err error
		switch choice {
		case workflowSDD:
			err = switchToSDD(svc)
		default:
			err = switchToODD(svc)
		}
		return workflowAppliedMsg{err: err}
	}
}

func switchToSDD(svc services) error {
	if err := removeBlock(svc.host, svc.backup, odd.MarkerStart, odd.MarkerEnd, "odd-remove"); err != nil {
		return err
	}
	if svc.host != nil {
		if err := cleanModules(scoped.Dir(svc.host.ConfigDir)); err != nil {
			return err
		}
	}
	if err := applySDD(svc.host, svc.state, svc.backup, sdd.DefaultAssignments(), sdd.DefaultEfforts(), false, nil); err != nil {
		return err
	}
	if svc.state != nil {
		return svc.state.SetWorkflow("sdd")
	}
	return nil
}

func switchToODD(svc services) error {
	if err := removeBlock(svc.host, svc.backup, sdd.MarkerStart, sdd.MarkerEnd, "sdd-remove"); err != nil {
		return err
	}
	triggerRules, modular := false, false
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil {
			triggerRules = st.TriggerRules
			modular = st.ModularODD
		}
	}
	if err := applyODD(svc.host, svc.state, svc.backup, triggerRules, modular); err != nil {
		return err
	}
	if svc.state != nil {
		return svc.state.SetWorkflow("odd")
	}
	return nil
}

func removeBlock(host *copilot.Host, bkp *backup.Store, markerStart, markerEnd, label string) error {
	if host == nil {
		return nil
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, markerStart, markerEnd, "")
	if err != nil || !changed {
		return err
	}
	if bkp != nil {
		if _, err := bkp.CreateFiles(label, Version, []string{path}); err != nil {
			return fmt.Errorf("backup failed: %w", err)
		}
	}
	return instructions.Write(path, content)
}

func (w *workflowScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Switch workflow") + "\n\n")

	switch w.state {
	case wfApplying:
		b.WriteString("Applying workflow change…\n")
	case wfDone:
		name := workflowChoices[w.current].name
		b.WriteString(okSty.Render(fmt.Sprintf("Switched to %s ✓", name)) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case wfFailed:
		b.WriteString(errSty.Render("Error: "+w.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	default:
		for i, c := range workflowChoices {
			marker := "  "
			if c.id == w.current {
				marker = okSty.Render("● ")
			}
			if i == w.cursor {
				b.WriteString(titleSty.Render(menuCursor+c.name) + " " + marker + "\n")
			} else {
				b.WriteString(textSty.Render("  "+c.name) + " " + marker + "\n")
			}
		}
		b.WriteString("\n")
		b.WriteString(dimSty.Render(workflowChoices[w.cursor].desc) + "\n\n")
		b.WriteString(dimSty.Render("↑/↓ move · enter apply · q back") + "\n")
	}
	return b.String()
}
