package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// fixtureChange writes a planning-complete change and returns its workspace root.
func fixtureChange(t *testing.T, name string) string {
	t.Helper()
	cwd := t.TempDir()
	root := filepath.Join(cwd, "openspec", "changes", name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"proposal.md": "# Proposal",
		"spec.md":     "# Spec",
		"design.md":   "# Design",
		"tasks.md":    "- [ ] 1. do it",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cwd
}

func TestParseSDDArgs(t *testing.T) {
	opts, jsonOut, err := parseSDDArgs([]string{"add-auth", "--cwd", "/repo", "--json"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.ChangeName != "add-auth" || opts.Cwd != "/repo" || !jsonOut {
		t.Errorf("got %+v json=%v", opts, jsonOut)
	}

	opts, _, err = parseSDDArgs([]string{"--cwd=/r2"})
	if err != nil || opts.Cwd != "/r2" {
		t.Errorf("--cwd= form: opts=%+v err=%v", opts, err)
	}

	if _, _, err := parseSDDArgs([]string{"--cwd"}); err == nil {
		t.Error("--cwd without a value should error")
	}
	if _, _, err := parseSDDArgs([]string{"--bogus"}); err == nil {
		t.Error("unknown flag should error")
	}
	if _, _, err := parseSDDArgs([]string{"a", "b"}); err == nil {
		t.Error("two positional change names should error")
	}
}

func TestSDDCommandStatusJSON(t *testing.T) {
	cwd := fixtureChange(t, "add-auth")
	var out bytes.Buffer
	handled, err := sddCommand("sdd-status", []string{"--cwd", cwd, "--json"}, &out)
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"schemaName": "capiko.sdd-status"`)) {
		t.Errorf("expected capiko.sdd-status JSON, got:\n%s", out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(`"nextRecommended": "apply"`)) {
		t.Errorf("expected nextRecommended apply, got:\n%s", out.String())
	}
}

func TestSDDCommandStatusMarkdown(t *testing.T) {
	cwd := fixtureChange(t, "add-auth")
	var out bytes.Buffer
	if _, err := sddCommand("sdd-status", []string{"--cwd", cwd}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("## SDD Status: add-auth")) {
		t.Errorf("expected markdown heading, got:\n%s", out.String())
	}
}

func TestSDDCommandContinue(t *testing.T) {
	cwd := fixtureChange(t, "add-auth")
	var out bytes.Buffer
	if _, err := sddCommand("sdd-continue", []string{"--cwd", cwd}, &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("Native SDD Dispatcher")) || !bytes.Contains(out.Bytes(), []byte("next_recommended: apply")) {
		t.Errorf("expected dispatcher routing, got:\n%s", out.String())
	}
}

func TestSDDCommandNotAnSDDCommand(t *testing.T) {
	var out bytes.Buffer
	handled, err := sddCommand("version", nil, &out)
	if handled || err != nil {
		t.Errorf("non-SDD command: handled=%v err=%v", handled, err)
	}
	if out.Len() != 0 {
		t.Errorf("should not write anything, got %q", out.String())
	}
}
