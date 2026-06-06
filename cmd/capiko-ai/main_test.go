package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
	"github.com/martinhg/capiko-ai/internal/skill"
	"github.com/martinhg/capiko-ai/internal/state"
)

func testCatalog() []skill.Skill {
	return []skill.Skill{
		{Name: "capiko-hello", Content: "---\nname: capiko-hello\n---\nx"},
		{Name: "capiko-pr", Content: "---\nname: capiko-pr\n---\nx"},
	}
}

func TestPostUpgradeSyncWritesCatalog(t *testing.T) {
	skillsDir := t.TempDir()
	store := state.NewStore(t.TempDir())
	cat := testCatalog()

	var out bytes.Buffer
	detect := func() (*copilot.Host, error) { return &copilot.Host{SkillsDir: skillsDir}, nil }
	postUpgradeSync(detect, cat, store, nil, &out)

	for _, sk := range cat {
		if _, err := os.Stat(filepath.Join(skillsDir, sk.Name, "SKILL.md")); err != nil {
			t.Errorf("%s not synced: %v", sk.Name, err)
		}
	}
	if !bytes.Contains(out.Bytes(), []byte("synced 2 skill")) {
		t.Errorf("missing summary, got %q", out.String())
	}
}

func TestPostUpgradeSyncSkipsWhenNoHost(t *testing.T) {
	var out bytes.Buffer
	postUpgradeSync(func() (*copilot.Host, error) { return nil, nil }, testCatalog(), nil, nil, &out)
	if out.Len() != 0 {
		t.Errorf("expected no output when Copilot is absent, got %q", out.String())
	}
}

func TestPostUpgradeSyncSkipsOnDetectError(t *testing.T) {
	var out bytes.Buffer
	postUpgradeSync(func() (*copilot.Host, error) { return nil, errors.New("boom") }, testCatalog(), nil, nil, &out)
	if out.Len() != 0 {
		t.Errorf("detect error should be a silent no-op, got %q", out.String())
	}
}
