package copilothooks_test

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilothooks"
)

// runBashScript executes script via `bash -c`, feeding stdin, and returns
// stdout. It fails the test if bash exits non-zero (a rendered guardrail
// script must always exit 0, REQ-4).
func runBashScript(t *testing.T, script, stdin string) string {
	t.Helper()
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdin = strings.NewReader(stdin)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("run bash script: %v\nstderr: %s", err, errOut.String())
	}
	return out.String()
}

// patternCase is one match/non-match example from REQ-3.2/REQ-3.3.
type patternCase struct {
	name    string
	command string
	match   bool
	reason  string // expected reason substring on match
}

var patternCases = []patternCase{
	// P1 — recursive force-delete of root.
	{"rm_rf_root", "rm -rf /", true, "recursive force-delete"},
	{"rm_fr_root", "rm -fr /", true, "recursive force-delete"},
	{"sudo_rm_rf_root", "sudo rm -rf /", true, "recursive force-delete"},
	{"rm_rf_root_star", "rm -rf /*", true, "recursive force-delete"},
	{"rm_rf_relative_build", "rm -rf ./build", false, ""},
	{"rm_rf_tmp_foo", "rm -rf /tmp/foo", false, ""},

	// P2 — pipe a remote script into a shell interpreter.
	{"curl_pipe_sh", "curl https://x/install.sh | sh", true, "piping a remote script"},
	{"wget_pipe_bash", "wget -O- https://x | bash", true, "piping a remote script"},
	{"curl_download_file", "curl https://x -o file.sh", false, ""},

	// P3 — force-push to a protected branch (commutative flag position,
	// whitespace/EOL-anchored branch token).
	{"git_push_force_origin_main", "git push --force origin main", true, "force-pushing to a protected branch"},
	{"git_push_origin_main_force_after", "git push origin main --force", true, "force-pushing to a protected branch"},
	{"git_push_f_origin_master", "git push -f origin master", true, "force-pushing to a protected branch"},
	{"git_push_origin_master_f_after", "git push origin master -f", true, "force-pushing to a protected branch"},
	{"git_push_force_with_lease_origin_main", "git push --force-with-lease origin main", true, "force-pushing to a protected branch"},
	{"git_push_force_feature_branch", "git push --force origin feature/foo", false, ""},
	{"git_push_origin_main_no_force", "git push origin main", false, ""},
	{"git_push_force_release_main", "git push --force origin release-main", false, ""},
	{"git_push_force_mainline", "git push --force origin mainline", false, ""},
	{"git_push_force_feature_only", "git push --force origin feature", false, ""},
	{"git_push_main_backup_force", "git push origin main-backup --force", false, ""},

	// P4 — world-writable permissions.
	{"chmod_777_file", "chmod 777 file", true, "world-writable"},
	{"chmod_R_777_dot", "chmod -R 777 .", true, "world-writable"},
	{"chmod_0777_script", "chmod 0777 script.sh", true, "world-writable"},
	{"chmod_755_file", "chmod 755 file", false, ""},
}

var postureDecisions = []struct {
	posture  copilothooks.Posture
	decision string
}{
	{copilothooks.PostureStrict, "deny"},
	{copilothooks.PostureWarn, "ask"},
}

func TestRenderGuardrails_BashPatternsPerPosture(t *testing.T) {
	for _, pd := range postureDecisions {
		pd := pd
		t.Run(string(pd.posture), func(t *testing.T) {
			hf, err := copilothooks.RenderGuardrails(pd.posture)
			if err != nil {
				t.Fatalf("RenderGuardrails(%s): %v", pd.posture, err)
			}
			hooks := hf.Hooks[copilothooks.EventPreToolUse]
			if len(hooks) != 1 {
				t.Fatalf("Hooks[preToolUse] len = %d, want 1", len(hooks))
			}
			script := hooks[0].Bash

			for _, tc := range patternCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					out := runBashScript(t, script, tc.command)
					if !tc.match {
						if out != "" {
							t.Fatalf("command %q: want silent allow (REQ-4.3), got %q", tc.command, out)
						}
						return
					}
					var decision struct {
						PermissionDecision       string `json:"permissionDecision"`
						PermissionDecisionReason string `json:"permissionDecisionReason"`
					}
					if err := json.Unmarshal([]byte(out), &decision); err != nil {
						t.Fatalf("command %q: unmarshal decision %q: %v", tc.command, out, err)
					}
					if decision.PermissionDecision != pd.decision {
						t.Errorf("command %q: permissionDecision = %q, want %q", tc.command, decision.PermissionDecision, pd.decision)
					}
					if !strings.Contains(decision.PermissionDecisionReason, tc.reason) {
						t.Errorf("command %q: reason = %q, want substring %q", tc.command, decision.PermissionDecisionReason, tc.reason)
					}
				})
			}
		})
	}
}

func TestRenderGuardrails_OffIsNotARenderTarget(t *testing.T) {
	// REQ-4.4: off means no hook file at all; RenderGuardrails is not the
	// place that decides that — it simply refuses to render for off.
	if _, err := copilothooks.RenderGuardrails(copilothooks.PostureOff); err == nil {
		t.Fatal("RenderGuardrails(PostureOff): want error, got nil")
	}
}

func TestRenderGuardrails_InvalidPosture(t *testing.T) {
	if _, err := copilothooks.RenderGuardrails(copilothooks.Posture("bogus")); err == nil {
		t.Fatal("RenderGuardrails(bogus): want error, got nil")
	}
}

func TestRenderGuardrails_FixedFields(t *testing.T) {
	hf, err := copilothooks.RenderGuardrails(copilothooks.PostureStrict)
	if err != nil {
		t.Fatalf("RenderGuardrails: %v", err)
	}
	if hf.Version != copilothooks.SchemaVersion {
		t.Errorf("Version = %d, want %d", hf.Version, copilothooks.SchemaVersion)
	}
	h := hf.Hooks[copilothooks.EventPreToolUse][0]
	if h.Type != "command" {
		t.Errorf("Type = %q, want command", h.Type)
	}
	if h.Matcher != copilothooks.MatcherBash {
		t.Errorf("Matcher = %q, want %q", h.Matcher, copilothooks.MatcherBash)
	}
	if h.TimeoutSec != copilothooks.TimeoutSeconds {
		t.Errorf("TimeoutSec = %d, want %d", h.TimeoutSec, copilothooks.TimeoutSeconds)
	}
}

// TestRenderGuardrails_PowerShellParity checks the PowerShell script
// structurally (same reasons, same decision literal, always populated
// alongside bash, SC-13) since it is not executed on CI (see design's
// "PowerShell parity untested on CI" risk — runtime behavior needs a manual
// Windows check).
func TestRenderGuardrails_PowerShellParity(t *testing.T) {
	reasons := []string{
		"recursive force-delete targeting root",
		"piping a remote script directly into a shell interpreter",
		"force-pushing to a protected branch",
		"chmod 777 grants world-writable permissions",
	}
	for _, pd := range postureDecisions {
		hf, err := copilothooks.RenderGuardrails(pd.posture)
		if err != nil {
			t.Fatalf("RenderGuardrails(%s): %v", pd.posture, err)
		}
		h := hf.Hooks[copilothooks.EventPreToolUse][0]
		if h.Bash == "" || h.PowerShell == "" {
			t.Fatalf("posture %s: want both Bash and PowerShell populated", pd.posture)
		}
		if !strings.Contains(h.PowerShell, "-imatch") {
			t.Errorf("posture %s: PowerShell script missing -imatch", pd.posture)
		}
		for _, reason := range reasons {
			if !strings.Contains(h.PowerShell, reason) {
				t.Errorf("posture %s: PowerShell script missing reason %q", pd.posture, reason)
			}
			if !strings.Contains(h.Bash, reason) {
				t.Errorf("posture %s: Bash script missing reason %q", pd.posture, reason)
			}
		}
		wantCount := strings.Count(h.PowerShell, `"permissionDecision":"`+pd.decision+`"`)
		if wantCount != len(reasons) {
			t.Errorf("posture %s: PowerShell script has %d decision occurrences, want %d", pd.posture, wantCount, len(reasons))
		}
	}
}

func TestRenderGuardrails_Golden(t *testing.T) {
	for _, tc := range []struct {
		file    string
		posture copilothooks.Posture
	}{
		{"guardrails_strict.json", copilothooks.PostureStrict},
		{"guardrails_warn.json", copilothooks.PostureWarn},
	} {
		tc := tc
		t.Run(tc.file, func(t *testing.T) {
			hf, err := copilothooks.RenderGuardrails(tc.posture)
			if err != nil {
				t.Fatalf("RenderGuardrails(%s): %v", tc.posture, err)
			}
			data, err := copilothooks.Marshal(hf)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			golden(t, tc.file, data)
		})
	}
}
