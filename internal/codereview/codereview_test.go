package codereview

import (
	"strings"
	"testing"
)

func TestRulesUsesActionKeywords(t *testing.T) {
	body := Rules("capiko")
	for _, kw := range []string{"REJECT", "REQUIRE", "PREFER"} {
		if !strings.Contains(body, kw) {
			t.Errorf("rules should use the %q action keyword gga understands:\n%s", kw, body)
		}
	}
	for _, section := range []string{"Architecture", "Naming", "Testing"} {
		if !strings.Contains(body, section) {
			t.Errorf("rules should cover the %q section:\n%s", section, body)
		}
	}
}

func TestRulesIncludesPersonaPointerWhenSet(t *testing.T) {
	if !strings.Contains(Rules("capiko"), "capiko") {
		t.Error("rules should point at the active persona when one is set")
	}
	// An empty persona name must not leave a dangling pointer line.
	if strings.Contains(Rules(""), "persona") {
		t.Errorf("rules should omit the persona pointer when no persona is set:\n%s", Rules(""))
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Provider != "claude" {
		t.Errorf("default provider = %q, want claude", c.Provider)
	}
	if c.RulesFile != "AGENTS.md" {
		t.Errorf("default rules file = %q, want AGENTS.md", c.RulesFile)
	}
	if !c.StrictMode {
		t.Error("strict mode should default on")
	}
	if c.Timeout != 300 {
		t.Errorf("default timeout = %d, want 300", c.Timeout)
	}
}

func TestRenderConfigContainsManagedSettings(t *testing.T) {
	out := RenderConfig(DefaultConfig())
	wants := []string{
		`PROVIDER="claude"`,
		`RULES_FILE="AGENTS.md"`,
		`STRICT_MODE="true"`,
		`TIMEOUT="300"`,
		`FILE_PATTERNS=`,
	}
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("rendered .gga missing %q:\n%s", w, out)
		}
	}
}

func TestRenderConfigReflectsProviderAndStrictMode(t *testing.T) {
	c := DefaultConfig()
	c.Provider = "ollama:llama3.2"
	c.StrictMode = false
	out := RenderConfig(c)
	if !strings.Contains(out, `PROVIDER="ollama:llama3.2"`) {
		t.Errorf("rendered .gga should reflect the chosen provider:\n%s", out)
	}
	if !strings.Contains(out, `STRICT_MODE="false"`) {
		t.Errorf("rendered .gga should reflect strict mode off:\n%s", out)
	}
}
