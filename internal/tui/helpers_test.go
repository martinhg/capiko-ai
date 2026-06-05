package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martinhg/capiko-ai/internal/skill"
)

// key builds the KeyMsg whose String() matches what the screens switch on.
func key(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// testCatalog mirrors the real catalog order: capiko-hello at index 0.
func testCatalog() []skill.Skill {
	return []skill.Skill{
		{Name: "capiko-hello", Description: "smoke test", Content: "---\nname: capiko-hello\n---\nx"},
		{Name: "capiko-conventions", Description: "conventions", Content: "---\nname: capiko-conventions\n---\nx"},
		{Name: "capiko-pr", Description: "pr", Content: "---\nname: capiko-pr\n---\nx"},
	}
}

func writeSkillFile(t *testing.T, skillsDir, name string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: "+name+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
