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
	"github.com/martinhg/capiko-ai/internal/headroom"
	"github.com/martinhg/capiko-ai/internal/instructions"
	"github.com/martinhg/capiko-ai/internal/state"
)

// headroomDetected is a test seam over headroom.Detected so the configure screen
// renders deterministically.
var headroomDetected = headroom.Detected

// applyHeadroom wires (enabled) or unwires (disabled) headroom's MCP server entry
// in Copilot's mcp-config.json, backing the file up only when the entry changes,
// then records the choice in state. capiko configures headroom; it never installs
// the binary. Shared by the configure screen and the post-sync re-apply, mirroring
// applyEngram.
func applyHeadroom(host *copilot.Host, store *state.Store, bkp *backup.Store, enabled bool) error {
	if host == nil {
		return nil
	}
	if !enabled {
		return disableHeadroom(host, store, bkp)
	}

	entry := headroom.CopilotCLIEntry()
	want := engram.EntryChecksum(entry)

	cur, ok := engram.MCPEntryChecksum(host.MCPConfigPath, headroom.ServerName)
	if !ok || cur != want {
		if err := backupMCPConfig(host, bkp); err != nil {
			return err
		}
		if err := engram.MergeMCPEntry(host.MCPConfigPath, "mcpServers", headroom.ServerName, entry); err != nil {
			return err
		}
	}

	// Pair the wiring with agent guidance, so Copilot actually uses the tools.
	if err := applyHeadroomGuidance(host, bkp, true); err != nil {
		return err
	}

	if store != nil {
		return store.SetHeadroom(&state.HeadroomRecord{Enabled: true, Checksum: want})
	}
	return nil
}

// applyHeadroomGuidance injects (enabled) or removes (disabled) the headroom usage
// block in Copilot's instructions file, backing up only when the file changes.
// Without the block the wired MCP server is never used; with it, Copilot routes
// bulky content through headroom. Mirrors applyTriggerRules.
func applyHeadroomGuidance(host *copilot.Host, bkp *backup.Store, enabled bool) error {
	if host.ConfigDir == "" {
		return nil // no instructions file to write without a config dir
	}
	var block string
	if enabled {
		block = headroom.Guidance()
	}
	path := filepath.Join(host.ConfigDir, "copilot-instructions.md")
	content, changed, err := instructions.Render(path, headroom.GuidanceMarkerStart, headroom.GuidanceMarkerEnd, block)
	if err != nil {
		return err
	}
	if changed {
		if bkp != nil {
			if _, err := bkp.CreateFiles("headroom", Version, []string{path}); err != nil {
				return fmt.Errorf("backup failed, aborting: %w", err)
			}
		}
		if err := instructions.Write(path, content); err != nil {
			return err
		}
	}
	return nil
}

// disableHeadroom removes headroom's MCP entry (backing up the config first) and
// records the disabled state, so toggling headroom off and applying fully removes
// the wiring.
func disableHeadroom(host *copilot.Host, store *state.Store, bkp *backup.Store) error {
	if err := backupMCPConfig(host, bkp); err != nil {
		return err
	}
	if err := engram.RemoveMCPEntry(host.MCPConfigPath, "mcpServers", headroom.ServerName); err != nil {
		return err
	}
	// Remove the paired agent guidance too, so disabling fully unwires headroom.
	if err := applyHeadroomGuidance(host, bkp, false); err != nil {
		return err
	}
	if store != nil {
		return store.SetHeadroom(&state.HeadroomRecord{Enabled: false})
	}
	return nil
}

// backupMCPConfig snapshots the Copilot mcp-config.json before a headroom mutation,
// but only when the file already exists (a first write has nothing to back up).
func backupMCPConfig(host *copilot.Host, bkp *backup.Store) error {
	if bkp == nil {
		return nil
	}
	if _, err := os.Stat(host.MCPConfigPath); err != nil {
		return nil
	}
	if _, err := bkp.CreateFiles("headroom", Version, []string{host.MCPConfigPath}); err != nil {
		return fmt.Errorf("backup failed, aborting: %w", err)
	}
	return nil
}

// headroomScreen wires the optional headroom context-compression layer into
// Copilot via its MCP server. It is intentionally simple — a single Enabled toggle
// plus Apply/Back — and, when the headroom CLI is not on PATH, shows the install
// command (capiko configures headroom, it never installs it).
type headroomScreen struct {
	svc      services
	enabled  bool
	detected bool
	cursor   int // 0 enabled, 1 Apply, 2 Back
	state    headroomScreenState
	err      error
}

type headroomScreenState int

const (
	headroomEditing headroomScreenState = iota
	headroomApplying
	headroomDone
	headroomFailed
)

type headroomAppliedMsg struct{ err error }

func newHeadroom(svc services) screen {
	s := &headroomScreen{svc: svc, detected: headroomDetected()}
	if svc.state != nil {
		if st, err := svc.state.Load(); err == nil && st.Headroom != nil {
			s.enabled = st.Headroom.Enabled
		}
	}
	return s
}

const headroomRows = 1 // enabled

func (s *headroomScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case headroomAppliedMsg:
		if msg.err != nil {
			s.state, s.err = headroomFailed, msg.err
			return s, nil
		}
		s.state = headroomDone
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return s, back
		}
		if s.state == headroomDone || s.state == headroomFailed {
			return s, back
		}
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < headroomRows+1 {
				s.cursor++
			}
		case "left", "h", "right", "l", " ":
			if s.cursor == 0 {
				s.enabled = !s.enabled
			}
		case "enter":
			switch s.cursor {
			case headroomRows: // Apply
				s.state = headroomApplying
				return s, s.applyCmd()
			case headroomRows + 1: // Back
				return s, back
			}
		}
	}
	return s, nil
}

func (s *headroomScreen) applyCmd() tea.Cmd {
	host, store, bkp, enabled := s.svc.host, s.svc.state, s.svc.backup, s.enabled
	return func() tea.Msg {
		return headroomAppliedMsg{err: applyHeadroom(host, store, bkp, enabled)}
	}
}

func (s *headroomScreen) View() string {
	var b strings.Builder
	b.WriteString(titleSty.Render("Configure headroom") + "\n\n")
	b.WriteString(dimSty.Render("Context compression for Copilot — fewer tokens, same answers. capiko wires") + "\n")
	b.WriteString(dimSty.Render("in headroom's MCP server; it never installs the tool for you.") + "\n\n")

	switch s.state {
	case headroomApplying:
		b.WriteString("Applying headroom configuration…\n")
		return b.String()
	case headroomDone:
		if s.enabled {
			b.WriteString(okSty.Render("headroom wired ✓") + "\n\n")
			b.WriteString(dimSty.Render("Copilot can now call headroom_compress / headroom_retrieve.") + "\n\n")
		} else {
			b.WriteString(okSty.Render("headroom unwired ✓") + "\n\n")
		}
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	case headroomFailed:
		b.WriteString(errSty.Render("Error: "+s.err.Error()) + "\n\n")
		b.WriteString(dimSty.Render("any key to go back") + "\n")
		return b.String()
	}

	enabledVal := dimSty.Render("off")
	if s.enabled {
		enabledVal = okSty.Render("on")
	}
	marker, nameSty := "  ", textSty
	if s.cursor == 0 {
		marker, nameSty = titleSty.Render(menuCursor), titleSty
	}
	fmt.Fprintf(&b, "%s%s  %s\n\n", marker, nameSty.Render(pad("Enabled", 14)), enabledVal)

	for i, opt := range []string{"Apply", "Back"} {
		m, optSty := "  ", textSty
		if s.cursor == headroomRows+i {
			m, optSty = titleSty.Render(menuCursor), titleSty
		}
		b.WriteString(m + optSty.Render(opt) + "\n")
	}

	if !s.detected {
		b.WriteString("\n" + warnSty.Render("headroom is not on PATH.") + "\n")
		b.WriteString(dimSty.Render(`Install it:  pip install "headroom-ai[all]"`) + "\n")
		b.WriteString(dimSty.Render("             (or npm i -g headroom-ai · uv tool install \"headroom-ai[all]\")") + "\n")
	}

	b.WriteString("\n" + dimSty.Render("↑/↓ move · ←/→/space toggle · enter select · esc back") + "\n")
	return b.String()
}
