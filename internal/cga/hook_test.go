package cga

import (
	"fmt"
	"strings"
	"testing"
)

// testLogPath is the logPath value used across findings-log tests. Its
// value never touches disk — RenderPreCommitHook/RenderPostCommitHook are
// pure string builders — so any path string is fine here.
const testLogPath = "/repo/.git/capiko/cga-findings.jsonl"

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

func TestPostCommitMarkersAreCommentDelimitedAndDistinct(t *testing.T) {
	if PostCommitMarkerStart == "" || PostCommitMarkerEnd == "" {
		t.Fatal("PostCommitMarkerStart/PostCommitMarkerEnd must not be empty")
	}
	if PostCommitMarkerStart == PostCommitMarkerEnd {
		t.Fatal("PostCommitMarkerStart and PostCommitMarkerEnd must differ")
	}
	if !strings.HasPrefix(PostCommitMarkerStart, "#") || !strings.HasPrefix(PostCommitMarkerEnd, "#") {
		t.Errorf("markers must be shell comments so githooks.WriteBlock can inject them into a bash script; got %q / %q", PostCommitMarkerStart, PostCommitMarkerEnd)
	}
	if PostCommitMarkerStart == MarkerStart || PostCommitMarkerEnd == MarkerEnd {
		t.Error("post-commit markers must differ from pre-commit markers so the two managed blocks never collide")
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
			script := RenderPreCommitHook(tt.rules, tt.strict, tt.timeout, testLogPath, 0)

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
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
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
		script := RenderPreCommitHook("RULES-BODY", true, timeout, testLogPath, 0)
		if !strings.Contains(script, "TIMEOUT=120") {
			t.Errorf("RenderHook with timeout=%d should default to 120s, got:\n%s", timeout, script)
		}
	}
}

func TestRenderHookRecursionGuard(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
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

func TestRenderHookDisplaysFindingsViaJq(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	if !strings.Contains(script, `.findings[]?`) {
		t.Errorf("rendered hook missing jq findings extraction (.findings[]?):\n%s", script)
	}
	if !strings.Contains(script, "severity") {
		t.Errorf("rendered hook findings block should reference severity:\n%s", script)
	}
}

func TestRenderHookFindingsDisplayBeforeCaseStatement(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	findingsIdx := strings.Index(script, `.findings[]?`)
	caseIdx := strings.Index(script, `case "$verdict"`)
	if findingsIdx < 0 || caseIdx < 0 {
		t.Fatal("expected both findings display and case statement in rendered hook")
	}
	if findingsIdx > caseIdx {
		t.Errorf("findings display must appear before the verdict case-statement (runs on PASS and FAIL):\n%s", script)
	}
}

func TestRenderHookIsBashOnlyNoPowerShell(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	for _, banned := range []string{"powershell", "PowerShell", "Get-ChildItem", "$env:"} {
		if strings.Contains(script, banned) {
			t.Errorf("rendered hook must be bash-only (Phase 0), found %q:\n%s", banned, script)
		}
	}
}

// findingsLogBlock extracts the "# cga: append findings log entry" ...
// "# cga: end findings log entry" section from a rendered script so tests
// can assert on it in isolation, without false positives from the rest of
// the hook. Fails the test if the markers are missing.
func findingsLogBlock(t *testing.T, script string) string {
	t.Helper()
	start := strings.Index(script, "# cga: append findings log entry")
	end := strings.Index(script, "# cga: end findings log entry")
	if start < 0 || end < 0 || end < start {
		t.Fatalf("expected a findings-log append block delimited by start/end comments:\n%s", script)
	}
	return script[start:end]
}

func TestRenderPreCommitHookAppendsFindingsLogEntry(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	block := findingsLogBlock(t, script)

	for _, want := range []string{
		testLogPath,              // logPath embedded
		`mkdir -p`,               // parent dir created when missing
		`jq -c`,                  // JSON entry composed via jq
		`"timestamp":$ts`,        // schema field (jq object literal, no spaces)
		`"parent_sha":$parent`,   // schema field
		`"commit_sha":"pending"`, // commit_sha starts pending
		`"verdict":$verdict`,     // schema field
		`>> "$logPath"`,          // appended, not overwritten
	} {
		if !strings.Contains(block, want) {
			t.Errorf("findings-log append block missing %q:\n%s", want, block)
		}
	}
}

func TestRenderPreCommitHookRotatesLogWithDefaultCap(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	block := findingsLogBlock(t, script)
	if !strings.Contains(block, "tail -n 200 ") {
		t.Errorf("expected rotation to default to 200 entries when rotationCap<=0:\n%s", block)
	}
}

func TestRenderPreCommitHookRotatesLogWithCustomCap(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 50)
	block := findingsLogBlock(t, script)
	if !strings.Contains(block, "tail -n 50 ") {
		t.Errorf("expected rotation to use the custom cap 50:\n%s", block)
	}
	if strings.Contains(block, "tail -n 200 ") {
		t.Errorf("custom cap must not fall back to the default 200:\n%s", block)
	}
}

func TestRenderPreCommitHookFindingsLogNeverExitsNonZeroOnFailure(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, testLogPath, 0)
	block := findingsLogBlock(t, script)

	if strings.Contains(block, "exit 1") {
		t.Errorf("findings-log append/rotate must never exit 1 — log failures must not block the commit:\n%s", block)
	}
	for _, want := range []string{"2>/dev/null", "|| true"} {
		if !strings.Contains(block, want) {
			t.Errorf("findings-log append block missing failure guard %q:\n%s", want, block)
		}
	}
}

func TestRenderPreCommitHookLoggingDisabledWhenLogPathEmpty(t *testing.T) {
	script := RenderPreCommitHook("RULES-BODY", true, 120, "", 0)
	if strings.Contains(script, "cga: append findings log entry") {
		t.Errorf("empty logPath must disable findings-log persistence entirely:\n%s", script)
	}
	if strings.Contains(script, "logPath=") {
		t.Errorf("empty logPath must not emit a logPath shell variable:\n%s", script)
	}
}

func TestRenderPostCommitHookPatchesPendingEntryViaTmpAndMv(t *testing.T) {
	script := RenderPostCommitHook(testLogPath)

	for _, want := range []string{
		testLogPath,
		"git rev-parse --git-dir", // worktree-safe git-dir resolution
		"git rev-parse HEAD",      // real commit SHA, available post-commit
		"mktemp",                  // tmp file, never sed -i
		`mv "$tmp_log" "$logPath"`,
		`jq -c --arg sha "$commit_sha" '.commit_sha=$sha'`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("post-commit hook missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "sed -i") {
		t.Errorf("post-commit hook must not use sed -i (BSD/GNU flag divergence):\n%s", script)
	}
}

func TestRenderPostCommitHookNoOpsWhenLastEntryAlreadyHasRealSHA(t *testing.T) {
	script := RenderPostCommitHook(testLogPath)

	// The script must gate the entire patch (tmp file creation, mv) behind
	// a check that the last line still carries commit_sha:"pending" —
	// otherwise an already-patched entry (real SHA) is left untouched.
	pendingCheckIdx := strings.Index(script, `"commit_sha":"pending"`)
	mvIdx := strings.Index(script, `mv "$tmp_log" "$logPath"`)
	if pendingCheckIdx < 0 || mvIdx < 0 {
		t.Fatal("expected both a pending-check and the tmp+mv patch in the rendered post-commit hook")
	}
	if pendingCheckIdx > mvIdx {
		t.Errorf("pending check must gate the patch, appearing before the tmp+mv write:\n%s", script)
	}
	if !strings.Contains(script, "*) exit 0 ;;") {
		t.Errorf("expected a case-statement default branch that exits 0 (no-op) when the last entry is not pending:\n%s", script)
	}
}

func TestRenderPostCommitHookEmptyLogPathReturnsEmptyScript(t *testing.T) {
	script := RenderPostCommitHook("")
	if script != "" {
		t.Errorf("empty logPath must disable the post-commit hook entirely, got:\n%s", script)
	}
}

func TestRenderPostCommitHookIsBashOnlyNoPowerShell(t *testing.T) {
	script := RenderPostCommitHook(testLogPath)
	for _, banned := range []string{"powershell", "PowerShell", "Get-ChildItem", "$env:"} {
		if strings.Contains(script, banned) {
			t.Errorf("rendered post-commit hook must be bash-only, found %q:\n%s", banned, script)
		}
	}
}
