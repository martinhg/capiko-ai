package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/martinhg/capiko-ai/internal/skill"
)

// key builds the KeyPressMsg whose String() matches what the screens switch on.
func key(s string) tea.KeyPressMsg {
	switch s {
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "space", " ":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
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
