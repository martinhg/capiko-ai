package skillregistry

import (
	"strings"
	"testing"
)

func TestRenderMarkdown(t *testing.T) {
	reg := Registry{
		Project: "capiko-ai",
		Sources: []string{"/home/u/.copilot/skills", "/work/proj/.copilot/skills"},
		Entries: []Entry{
			{Name: "alpha", Description: "Alpha skill. Trigger: a", Scope: "user", Path: "/home/u/.copilot/skills/alpha/SKILL.md"},
			{Name: "gamma", Description: "Gamma skill. Trigger: g", Scope: "project", Path: "/work/proj/.copilot/skills/gamma/SKILL.md"},
		},
	}

	out := RenderMarkdown(reg)

	for _, want := range []string{
		"# Skill Registry — capiko-ai",
		"## Sources scanned",
		"/home/u/.copilot/skills",
		"/work/proj/.copilot/skills",
		"## Contract",
		"Delegator use only",
		"## Skills",
		"| Skill | Trigger / description | Scope | Path |",
		"`alpha`",
		"Alpha skill. Trigger: a",
		"`/home/u/.copilot/skills/alpha/SKILL.md`",
		"project",
		"`gamma`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderMarkdown output missing %q\n---\n%s", want, out)
		}
	}
}

func TestRenderMarkdownEmpty(t *testing.T) {
	reg := Registry{Project: "empty", Sources: []string{"/a/b"}}
	out := RenderMarkdown(reg)
	if !strings.Contains(out, "# Skill Registry — empty") {
		t.Errorf("missing heading: %s", out)
	}
	// Still renders the table header even with no rows.
	if !strings.Contains(out, "| Skill | Trigger / description | Scope | Path |") {
		t.Errorf("missing table header: %s", out)
	}
}
