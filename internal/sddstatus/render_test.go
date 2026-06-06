package sddstatus

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderJSONMatchesContract(t *testing.T) {
	cwd := change(t, "add-auth", coreArtifacts())
	st, _ := Resolve(ResolveOptions{Cwd: cwd})

	out, err := RenderJSON(st)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	// It must be valid JSON carrying the capiko schema identity.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("RenderJSON produced invalid JSON: %v", err)
	}
	if decoded["schemaName"] != "capiko.sdd-status" {
		t.Errorf("schemaName = %v, want capiko.sdd-status", decoded["schemaName"])
	}
	if decoded["artifactStore"] != "openspec" {
		t.Errorf("artifactStore = %v, want openspec", decoded["artifactStore"])
	}
	if decoded["nextRecommended"] != "apply" {
		t.Errorf("nextRecommended = %v, want apply", decoded["nextRecommended"])
	}
	ph, ok := decoded["planningHome"].(map[string]any)
	if !ok || !strings.HasSuffix(ph["path"].(string), "/openspec") {
		t.Errorf("planningHome = %v, want a path ending in /openspec", decoded["planningHome"])
	}
}

func TestRenderJSONEmptyPathsAreArraysNotNull(t *testing.T) {
	// The contract requires empty path/array fields to be arrays, not null
	// (changeName and changeRoot are the only nullable fields).
	st, _ := Resolve(ResolveOptions{Cwd: t.TempDir()}) // no changes → blocked
	out, _ := RenderJSON(st)
	for _, arr := range []string{
		`"proposal": []`, `"specs": []`, `"design": []`, `"tasks": []`,
		`"applyProgress": []`, `"verifyReport": []`,
		`"allowedEditRoots": [`, `"blockedReasons": [`,
	} {
		if !strings.Contains(out, arr) {
			t.Errorf("expected %s to serialize as an array\n%s", arr, out)
		}
	}
}

func TestRenderMarkdownHasSummaryAndNext(t *testing.T) {
	cwd := change(t, "add-auth", coreArtifacts())
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	md := RenderMarkdown(st)
	for _, want := range []string{"## SDD Status: add-auth", "next: apply", "apply: ready", "```json"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestRenderDispatcherHasRouting(t *testing.T) {
	cwd := change(t, "add-auth", coreArtifacts())
	st, _ := Resolve(ResolveOptions{Cwd: cwd})
	md := RenderDispatcherMarkdown(st)
	for _, want := range []string{"next_recommended: apply", "### Dependency States", "- apply: ready"} {
		if !strings.Contains(md, want) {
			t.Errorf("dispatcher missing %q\n%s", want, md)
		}
	}
}
