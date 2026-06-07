package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/persona"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

// RunSync writes every catalog skill and agent to disk (overwriting), snapshotting
// the affected skills first and recording the result in state. It is the headless
// core shared by the Sync screen and the post-upgrade sync in main. A nil store,
// backup, or agentCatalog degrades gracefully. Returns the total number of items
// written (skills + agents).
func RunSync(host *copilot.Host, catalog []skill.Skill, agentCatalog []agent.Agent, store *state.Store, bkp *backup.Store) (int, error) {
	if bkp != nil {
		names := make([]string, len(catalog))
		for i, sk := range catalog {
			names[i] = sk.Name
		}
		if _, err := bkp.Create(host.SkillsDir, "sync", Version, names); err != nil {
			return 0, fmt.Errorf("backup failed, aborting: %w", err)
		}
	}

	// Install skills.
	recorded := make([]state.Installed, 0, len(catalog))
	for _, sk := range catalog {
		if _, err := sk.Install(host.SkillsDir); err != nil {
			return 0, fmt.Errorf("syncing %s: %w", sk.Name, err)
		}
		recorded = append(recorded, state.Installed{Name: sk.Name, Checksum: state.Checksum(sk.CanonicalContent())})
	}

	// Install agents alongside skills.
	agentRecorded := make([]state.Installed, 0, len(agentCatalog))
	for _, a := range agentCatalog {
		if _, err := a.Install(host.AgentsDir); err != nil {
			return 0, fmt.Errorf("syncing agent %s: %w", a.Name, err)
		}
		agentRecorded = append(agentRecorded, state.Installed{Name: a.Name, Checksum: state.Checksum(a.CanonicalContent())})
	}

	if store != nil {
		if err := store.Apply(Version, recorded, nil); err != nil {
			return 0, fmt.Errorf("recording state: %w", err)
		}
		if len(agentRecorded) > 0 {
			if err := store.ApplyAgents(Version, agentRecorded, nil); err != nil {
				return 0, fmt.Errorf("recording agent state: %w", err)
			}
		}
		// Re-apply the managed instruction blocks so they track the current
		// catalog/version (capiko's InjectForSync equivalent).
		if st, err := store.Load(); err == nil {
			if st.Persona != "" {
				if p, ok := persona.ByID(persona.ID(st.Persona)); ok {
					if err := applyPersona(host, store, bkp, p); err != nil {
						return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying persona: %w", err)
					}
				}
			}
			if len(st.SDDModels) > 0 || st.StrictTDD {
				if err := applySDD(host, store, bkp, st.SDDModels, st.StrictTDD); err != nil {
					return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying SDD: %w", err)
				}
			}
			// Re-apply curated scoped instructions so they track the catalog, into
			// the home dir and any COPILOT_CUSTOM_INSTRUCTIONS_DIRS — only once the
			// user has installed them, mirroring the persona/SDD opt-in above.
			if st.InstructionsInstalled {
				if _, err := installInstructions(host, bkp, store); err != nil {
					return len(recorded) + len(agentRecorded), fmt.Errorf("re-applying scoped instructions: %w", err)
				}
			}
		}
	}
	return len(recorded) + len(agentRecorded), nil
}

// syncScreen writes every catalog skill and agent to disk, overwriting so the
// installed items match the current catalog exactly ("sync configs").
type syncScreen struct {
	svc          services
	catalog      []skill.Skill
	agentCatalog []agent.Agent
	state        syncState
	count        int
	skillNames   []string // names of skills written in the last sync
	agentNames   []string // names of agents written in the last sync
	err          error
}

type syncState int

const (
	syncConfirm syncState = iota
	syncApplying
	syncDone
	syncFailed
)

type syncedMsg struct {
	count      int
	skillNames []string // names of skills written during this sync
	agentNames []string // names of agents written during this sync
	err        error
}

func newSync(svc services, catalog []skill.Skill, agentCatalog []agent.Agent) screen {
	return &syncScreen{svc: svc, catalog: catalog, agentCatalog: agentCatalog}
}

func (s *syncScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case syncedMsg:
		if msg.err != nil {
			s.state, s.err = syncFailed, msg.err
			return s, nil
		}
		s.state = syncDone
		s.count = msg.count
		s.skillNames = msg.skillNames
		s.agentNames = msg.agentNames
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "n":
			return s, back
		}
		if s.state == syncConfirm && (msg.String() == "y" || msg.String() == "enter") {
			s.state = syncApplying
			return s, s.syncCmd()
		}
		if s.state == syncDone || s.state == syncFailed {
			return s, back
		}
	}
	return s, nil
}

func (s *syncScreen) syncCmd() tea.Cmd {
	svc, cat, agentCat := s.svc, s.catalog, s.agentCatalog
	return func() tea.Msg {
		n, err := RunSync(svc.host, cat, agentCat, svc.state, svc.backup)
		if err != nil {
			return syncedMsg{err: err}
		}
		skillNames := make([]string, len(cat))
		for i, sk := range cat {
			skillNames[i] = sk.Name
		}
		agentNames := make([]string, len(agentCat))
		for i, ag := range agentCat {
			agentNames[i] = ag.Name
		}
		return syncedMsg{count: n, skillNames: skillNames, agentNames: agentNames}
	}
}

func (s *syncScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Sync configs") + "\n\n")

	switch s.state {
	case syncApplying:
		b.WriteString("Writing all catalog skills…\n")
	case syncDone:
		if len(s.skillNames) > 0 {
			b.WriteString(titleSty.Render("Skills") + "\n")
			for _, name := range s.skillNames {
				b.WriteString(okSty.Render("  ✓ ") + name + "\n")
			}
			b.WriteString("\n")
		}
		if len(s.agentNames) > 0 {
			b.WriteString(titleSty.Render("Agents") + "\n")
			for _, name := range s.agentNames {
				b.WriteString(okSty.Render("  ✓ ") + name + "\n")
			}
			b.WriteString("\n")
		}
		if len(s.skillNames) == 0 && len(s.agentNames) == 0 {
			b.WriteString(okSty.Render(fmt.Sprintf("Synced %d item(s) ✓", s.count)) + "\n\n")
		}
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	case syncFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
	default:
		fmt.Fprintf(&b, "Write all %d catalog skill(s) to\n%s,\noverwriting to match the catalog?\n\n", len(s.catalog), s.svc.host.SkillsDir)
		b.WriteString(dimSty.Render("[y to sync · q to go back]") + "\n")
	}
	return b.String()
}
