package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/backup"
	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/engram"
	"github.com/martinhg/capiko-ai/internal/state"
)

// Cloud client seams, swapped in tests so apply never shells out to the real
// engram binary.
var (
	cloudConfig = engram.CloudConfig
	cloudEnroll = engram.CloudEnroll
)

// applyEngram writes the engram MCP server entry into Copilot's mcp-config.json
// (merging, never clobbering other servers), backing the file up only when the
// entry changes, then records the configuration in state. Shared by the configure
// screen and the post-sync re-apply.
func applyEngram(host *copilot.Host, store *state.Store, bkp *backup.Store, rec *state.EngramRecord) error {
	if host == nil || rec == nil {
		return nil
	}
	entry := engram.CopilotCLIEntry(rec.CloudServer)
	want := engram.EntryChecksum(entry)

	cur, ok := engram.CLIEntryChecksum(host.MCPConfigPath)
	if !ok || cur != want {
		if bkp != nil {
			if _, err := os.Stat(host.MCPConfigPath); err == nil {
				if _, err := bkp.CreateFiles("engram", Version, []string{host.MCPConfigPath}); err != nil {
					return fmt.Errorf("backup failed, aborting: %w", err)
				}
			}
		}
		if err := engram.MergeMCPEntry(host.MCPConfigPath, "mcpServers", "engram", entry); err != nil {
			return err
		}
	}

	rec.Checksum = want
	if store != nil {
		return store.SetEngram(rec)
	}
	return nil
}

// applyEngramConfig wires engram for the workspace: it writes the per-repo project
// config, the MCP entry (with the cloud env when a server is set) and records the
// state, then points the cloud client at the server and enrolls the project. The
// workspace's base name is the project name.
func applyEngramConfig(svc services, workspace string, rec *state.EngramRecord) error {
	if svc.host == nil || rec == nil {
		return nil
	}
	project := filepath.Base(workspace)
	rec.Projects = []string{project}
	if len(rec.Surfaces) == 0 {
		rec.Surfaces = []string{"cli"}
	}
	if !rec.Enabled {
		return disableEngram(svc, workspace, rec)
	}
	if err := engram.WriteProjectConfig(workspace, project); err != nil {
		return err
	}
	if err := applyEngram(svc.host, svc.state, svc.backup, rec); err != nil {
		return err
	}
	if hasSurface(rec.Surfaces, "vscode") {
		path := filepath.Join(workspace, ".vscode", "mcp.json")
		if err := engram.MergeMCPEntry(path, "servers", "engram", engram.VSCodeEntry(rec.CloudServer)); err != nil {
			return err
		}
	}
	if rec.CloudServer != "" {
		if err := cloudConfig(rec.CloudServer); err != nil {
			return fmt.Errorf("engram cloud config: %w", err)
		}
		if err := cloudEnroll(project); err != nil {
			return fmt.Errorf("engram cloud enroll: %w", err)
		}
	}
	return nil
}

// disableEngram unwires engram: it removes the MCP entry from both surfaces
// (backing up the Copilot CLI config first) and records the disabled state, so
// toggling engram off and applying fully removes the wiring.
func disableEngram(svc services, workspace string, rec *state.EngramRecord) error {
	if svc.backup != nil {
		if _, err := os.Stat(svc.host.MCPConfigPath); err == nil {
			if _, err := svc.backup.CreateFiles("engram", Version, []string{svc.host.MCPConfigPath}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
	}
	if err := engram.RemoveMCPEntry(svc.host.MCPConfigPath, "mcpServers", "engram"); err != nil {
		return err
	}
	if err := engram.RemoveMCPEntry(filepath.Join(workspace, ".vscode", "mcp.json"), "servers", "engram"); err != nil {
		return err
	}
	rec.Checksum = ""
	if svc.state != nil {
		return svc.state.SetEngram(rec)
	}
	return nil
}

// hasSurface reports whether surfaces contains the named surface.
func hasSurface(surfaces []string, name string) bool {
	for _, s := range surfaces {
		if s == name {
			return true
		}
	}
	return false
}

// engramScreen enables and configures the engram backend: a toggle, the
// artifact-store mode, and the (optional) Engram Cloud server URL. Apply writes
// the MCP entry + per-repo project config and points the cloud client at the
// server. Disabling and applying leaves the recorded config but turns sync
// re-application off.
type engramScreen struct {
	svc     services
	enabled bool
	mode    string
	server  string
	vscode  bool // also wire the VS Code surface (.vscode/mcp.json)
	cursor  int  // 0 enabled, 1 mode, 2 server, 3 vscode, 4 Apply, 5 Back
	editing bool
	editBuf string
	state   engramState
	err     error
}

type engramState int

const (
	engramEditing engramState = iota
	engramApplying
	engramDone
	engramFailed
)

type engramAppliedMsg struct{ err error }

func newEngram(svc services) screen {
	s := &engramScreen{svc: svc, mode: engram.DefaultMode}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.Engram != nil {
			s.enabled = st.Engram.Enabled
			if st.Engram.ArtifactMode != "" {
				s.mode = st.Engram.ArtifactMode
			}
			s.server = st.Engram.CloudServer
			s.vscode = hasSurface(st.Engram.Surfaces, "vscode")
		}
	}
	return s
}

const engramRows = 4 // enabled, mode, server, vscode

func (s *engramScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case engramAppliedMsg:
		if msg.err != nil {
			s.state, s.err = engramFailed, msg.err
			return s, nil
		}
		s.state = engramDone
		return s, nil
	case tea.KeyMsg:
		if s.editing {
			return s.handleEdit(msg)
		}
		switch msg.String() {
		case "q", "esc":
			return s, back
		}
		if s.state == engramDone || s.state == engramFailed {
			return s, back
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < engramRows+1 {
				s.cursor++
			}
		case "left", "h", "right", "l", " ":
			s.adjust(msg.String())
		case "c":
			if s.cursor == 2 {
				s.editing, s.editBuf = true, s.server
			}
		case "enter":
			switch s.cursor {
			case engramRows: // Apply
				s.state = engramApplying
				return s, s.applyCmd()
			case engramRows + 1: // Back
				return s, back
			}
		}
	}
	return s, nil
}

func (s *engramScreen) adjust(key string) {
	switch s.cursor {
	case 0:
		s.enabled = !s.enabled
	case 1:
		s.cycleMode(key == "left" || key == "h")
	case 3:
		s.vscode = !s.vscode
	}
}

func (s *engramScreen) cycleMode(back bool) {
	idx := 0
	for i, m := range engram.Modes {
		if m == s.mode {
			idx = i
			break
		}
	}
	n := len(engram.Modes)
	delta := 1
	if back {
		delta = -1
	}
	s.mode = engram.Modes[((idx+delta)%n+n)%n]
}

func (s *engramScreen) handleEdit(msg tea.KeyMsg) (screen, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		s.server = strings.TrimSpace(s.editBuf)
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

func (s *engramScreen) applyCmd() tea.Cmd {
	svc := s.svc
	surfaces := []string{"cli"}
	if s.vscode {
		surfaces = append(surfaces, "vscode")
	}
	rec := &state.EngramRecord{Enabled: s.enabled, ArtifactMode: s.mode, CloudServer: s.server, Surfaces: surfaces}
	return func() tea.Msg {
		wd, err := os.Getwd()
		if err != nil {
			return engramAppliedMsg{err: err}
		}
		return engramAppliedMsg{err: applyEngramConfig(svc, wd, rec)}
	}
}

func (s *engramScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure engram") + "\n\n")
	b.WriteString(dimSty.Render("Cross-session memory for Copilot. Default mode hybrid: specs to git, memory to cloud.") + "\n\n")

	switch s.state {
	case engramApplying:
		b.WriteString("Applying engram configuration…\n")
		return b.String()
	case engramDone:
		b.WriteString(okSty.Render("engram configured ✓") + "\n\n")
		if s.server != "" {
			b.WriteString(dimSty.Render("Set ENGRAM_CLOUD_TOKEN in your environment; see docs/engram-cloud-setup.md.") + "\n\n")
		}
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case engramFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	enabledVal := dimSty.Render("off")
	if s.enabled {
		enabledVal = okSty.Render("on")
	}
	server := s.server
	if s.editing && s.cursor == 2 {
		server = s.editBuf + "▏"
	} else if server == "" {
		server = dimSty.Render("(local only — none)")
	}
	vscodeVal := dimSty.Render("off")
	if s.vscode {
		vscodeVal = okSty.Render("on")
	}
	rows := []struct{ label, val string }{
		{"Enabled", enabledVal},
		{"Mode", dimSty.Render(s.mode)},
		{"Cloud server", server},
		{"VS Code", vscodeVal},
	}
	for i, r := range rows {
		marker, nameSty := "  ", textSty
		if i == s.cursor {
			marker, nameSty = titleSty.Render(menuCursor), titleSty
		}
		fmt.Fprintf(&b, "%s%s  %s\n", marker, nameSty.Render(pad(r.label, 14)), r.val)
	}
	b.WriteString("\n")

	for i, opt := range []string{"Apply", "Back"} {
		marker, optSty := "  ", textSty
		if s.cursor == engramRows+i {
			marker, optSty = titleSty.Render(menuCursor), titleSty
		}
		b.WriteString(marker + optSty.Render(opt) + "\n")
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · ←/→/space change · c edit server · enter select · esc back") + "\n")
	return b.String()
}
