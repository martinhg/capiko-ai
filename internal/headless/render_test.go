package headless

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/tui"
)

func TestRenderTextEmptyResult(t *testing.T) {
	r := CommandResult{OK: true, Command: "sync"}
	var buf bytes.Buffer
	RenderText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "capiko-ai sync") {
		t.Errorf("missing header line:\n%s", out)
	}
	if !strings.Contains(out, "No drift detected, nothing to sync.") {
		t.Errorf("empty sync result should render the no-drift message:\n%s", out)
	}
}

func TestRenderTextInstallOnly(t *testing.T) {
	r := CommandResult{
		OK:      true,
		Command: "install",
		Items: ItemChanges{
			InstalledSkills: []string{"capiko-dev", "go-testing"},
			InstalledAgents: []string{"capiko-onboard"},
		},
		TotalChanged: 3,
	}
	var buf bytes.Buffer
	RenderText(&buf, r)
	out := buf.String()

	for _, want := range []string{
		"capiko-ai install",
		"Skills",
		"+ capiko-dev",
		"+ go-testing",
		"Agents",
		"+ capiko-onboard",
		"3 item(s) installed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "- ") {
		t.Errorf("install-only result should not contain removal markers:\n%s", out)
	}
}

func TestRenderTextUninstallOnly(t *testing.T) {
	r := CommandResult{
		OK:      true,
		Command: "uninstall",
		Items: ItemChanges{
			RemovedSkills: []string{"capiko-dev"},
			RemovedAgents: []string{"capiko-onboard"},
		},
		TotalChanged: 2,
	}
	var buf bytes.Buffer
	RenderText(&buf, r)
	out := buf.String()

	for _, want := range []string{
		"capiko-ai uninstall",
		"Skills",
		"- capiko-dev",
		"Agents",
		"- capiko-onboard",
		"2 item(s) removed.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "+ ") {
		t.Errorf("uninstall-only result should not contain install markers:\n%s", out)
	}
}

func TestRenderTextMixedSkillsAndAgents(t *testing.T) {
	r := CommandResult{
		OK:      true,
		Command: "sync",
		Items: ItemChanges{
			InstalledSkills: []string{"capiko-dev"},
			RemovedSkills:   []string{"old-skill"},
			InstalledAgents: []string{"capiko-onboard"},
			RemovedAgents:   []string{"old-agent"},
		},
		TotalChanged: 4,
	}
	var buf bytes.Buffer
	RenderText(&buf, r)
	out := buf.String()

	for _, want := range []string{"+ capiko-dev", "- old-skill", "+ capiko-onboard", "- old-agent"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderTextError(t *testing.T) {
	r := CommandResult{
		OK:      false,
		Command: "install",
		Error:   "GitHub Copilot CLI not found",
	}
	var buf bytes.Buffer
	RenderText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "GitHub Copilot CLI not found") {
		t.Errorf("error message missing from text output:\n%s", out)
	}
	if !strings.Contains(out, "capiko-ai install") {
		t.Errorf("missing header line:\n%s", out)
	}
}

func TestRenderJSONSchema(t *testing.T) {
	r := CommandResult{
		OK:      true,
		Command: "install",
		Items: ItemChanges{
			InstalledSkills: []string{"capiko-dev"},
			InstalledAgents: []string{"capiko-onboard"},
		},
		TotalChanged: 2,
	}

	var buf bytes.Buffer
	if err := RenderJSON(&buf, r); err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("RenderJSON output is not valid JSON: %v\noutput:\n%s", err, buf.String())
	}

	if decoded["ok"] != true {
		t.Errorf("ok = %v, want true", decoded["ok"])
	}
	if decoded["command"] != "install" {
		t.Errorf("command = %v, want install", decoded["command"])
	}
	if decoded["total_changed"] != float64(2) {
		t.Errorf("total_changed = %v, want 2", decoded["total_changed"])
	}
	if decoded["error"] != "" {
		t.Errorf("error = %v, want empty string", decoded["error"])
	}
}

func TestRenderJSONErrorField(t *testing.T) {
	r := CommandResult{
		OK:      false,
		Command: "sync",
		Error:   "boom",
	}
	var buf bytes.Buffer
	if err := RenderJSON(&buf, r); err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded["ok"] != false {
		t.Errorf("ok = %v, want false", decoded["ok"])
	}
	if decoded["error"] != "boom" {
		t.Errorf("error = %v, want boom", decoded["error"])
	}
}

func TestFromReconcileResultSuccess(t *testing.T) {
	rr := tui.ReconcileResult{
		InstalledSkills: []string{"capiko-dev"},
		InstalledAgents: []string{"capiko-onboard"},
	}
	r := FromReconcileResult("install", rr, nil)

	if !r.OK {
		t.Error("OK should be true when err is nil")
	}
	if r.Command != "install" {
		t.Errorf("Command = %q, want install", r.Command)
	}
	if len(r.Items.InstalledSkills) != 1 || r.Items.InstalledSkills[0] != "capiko-dev" {
		t.Errorf("Items.InstalledSkills = %v", r.Items.InstalledSkills)
	}
	if len(r.Items.InstalledAgents) != 1 || r.Items.InstalledAgents[0] != "capiko-onboard" {
		t.Errorf("Items.InstalledAgents = %v", r.Items.InstalledAgents)
	}
	if r.TotalChanged != 2 {
		t.Errorf("TotalChanged = %d, want 2", r.TotalChanged)
	}
	if r.Error != "" {
		t.Errorf("Error = %q, want empty", r.Error)
	}
}

func TestFromReconcileResultError(t *testing.T) {
	r := FromReconcileResult("uninstall", tui.ReconcileResult{}, errors.New("disk full"))

	if r.OK {
		t.Error("OK should be false when err is non-nil")
	}
	if r.Command != "uninstall" {
		t.Errorf("Command = %q, want uninstall", r.Command)
	}
	if r.Error != "disk full" {
		t.Errorf("Error = %q, want disk full", r.Error)
	}
	if r.TotalChanged != 0 {
		t.Errorf("TotalChanged = %d, want 0", r.TotalChanged)
	}
}
