package cga

import (
	"fmt"
	"strings"
	"testing"
)

func TestMarkersAreCommentDelimited(t *testing.T) {
	if MarkerStart == "" || MarkerEnd == "" {
		t.Fatal("MarkerStart/MarkerEnd must not be empty")
	}
	if MarkerStart == MarkerEnd {
		t.Fatal("MarkerStart and MarkerEnd must differ")
	}
	if !strings.HasPrefix(MarkerStart, "#") || !strings.HasPrefix(MarkerEnd, "#") {
		t.Errorf("markers must be shell comments so githooks.WriteBlock can inject them into a bash script; got %q / %q", MarkerStart, MarkerEnd)
	}
}

func TestRenderHookInvokesCopilotAndParsesVerdict(t *testing.T) {
	tests := []struct {
		name    string
		rules   string
		strict  bool
		timeout int
	}{
		{name: "strict mode on", rules: "RULES-BODY", strict: true, timeout: 120},
		{name: "strict mode off", rules: "RULES-BODY", strict: false, timeout: 45},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := RenderHook(tt.rules, tt.strict, tt.timeout)

			for _, want := range []string{
				"copilot -p --output-format json", // invocation
				"$STRICT",                         // strict var used
				"$TIMEOUT",                        // timeout var used
				"jq -r",                           // jq parse
				"grep -o",                         // grep fallback
				`-z "$diff"`,                      // empty-diff short-circuit condition
				"git diff --cached",               // staged diff collection
			} {
				if !strings.Contains(script, want) {
					t.Errorf("rendered hook missing %q:\n%s", want, script)
				}
			}

			wantStrict := "STRICT=false"
			if tt.strict {
				wantStrict = "STRICT=true"
			}
			if !strings.Contains(script, wantStrict) {
				t.Errorf("rendered hook missing %q for strict=%v:\n%s", wantStrict, tt.strict, script)
			}

			wantTimeout := fmt.Sprintf("TIMEOUT=%d", tt.timeout)
			if !strings.Contains(script, wantTimeout) {
				t.Errorf("rendered hook missing %q:\n%s", wantTimeout, script)
			}

			if !strings.Contains(script, tt.rules) {
				t.Errorf("rendered hook does not embed the rules text:\n%s", script)
			}
		})
	}
}

func TestRenderHookEmptyDiffShortCircuitsBeforeCopilot(t *testing.T) {
	script := RenderHook("RULES-BODY", true, 120)
	shortCircuitIdx := strings.Index(script, `-z "$diff"`)
	copilotIdx := strings.Index(script, "copilot -p")
	if shortCircuitIdx < 0 || copilotIdx < 0 {
		t.Fatal("expected both empty-diff check and copilot invocation in rendered hook")
	}
	if shortCircuitIdx > copilotIdx {
		t.Errorf("empty-diff short-circuit must appear before the copilot invocation:\n%s", script)
	}
}

func TestRenderHookDefaultsTimeoutWhenNonPositive(t *testing.T) {
	for _, timeout := range []int{0, -5} {
		script := RenderHook("RULES-BODY", true, timeout)
		if !strings.Contains(script, "TIMEOUT=120") {
			t.Errorf("RenderHook with timeout=%d should default to 120s, got:\n%s", timeout, script)
		}
	}
}

func TestRenderHookRecursionGuard(t *testing.T) {
	script := RenderHook("RULES-BODY", true, 120)
	guardIdx := strings.Index(script, "CGA_RUNNING")
	copilotIdx := strings.Index(script, "copilot -p")
	if guardIdx < 0 {
		t.Fatal("rendered hook must include a CGA_RUNNING recursion guard")
	}
	if copilotIdx < 0 {
		t.Fatal("rendered hook must include copilot invocation")
	}
	if guardIdx > copilotIdx {
		t.Error("recursion guard must appear before the copilot invocation")
	}
	if !strings.Contains(script, "export CGA_RUNNING=1") {
		t.Error("rendered hook must export CGA_RUNNING=1 so nested hooks see it")
	}
}

func TestRenderHookIsBashOnlyNoPowerShell(t *testing.T) {
	script := RenderHook("RULES-BODY", true, 120)
	for _, banned := range []string{"powershell", "PowerShell", "Get-ChildItem", "$env:"} {
		if strings.Contains(script, banned) {
			t.Errorf("rendered hook must be bash-only (Phase 0), found %q:\n%s", banned, script)
		}
	}
}
