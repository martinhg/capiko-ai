package githooks_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/martinhg/capiko-ai/internal/githooks"
)

// TestWriteBlock_SubshellPreservesCGASemantics actually executes the hook
// script produced by WriteBlock via `sh` — not just asserting on the
// rendered bytes — to prove the subshell wrapper (added to fix the
// multi-block dead-code hazard, see design R-3 Delivery Gates PR1) does not
// change CGA's existing control-flow semantics: the `$CGA_RUNNING`
// reentry-guard `exit`, the `export` that a later branch reads back via
// `$?`/`case`, and CGA's own `exit 0`/`exit 1` verdict branches all still
// behave the same inside a subshell as they did unwrapped. It also proves
// the composition property the wrapping exists for: a block written after
// CGA's still runs, and a failing block still halts the whole hook.
func TestWriteBlock_SubshellPreservesCGASemantics(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available in PATH")
	}

	// cgaLikeBlock mirrors the real CGA pre-commit block's shape: a
	// reentry guard (`exit 0` bare, no subshell of its own), an `export`
	// that later logic in the SAME block reads back, and a case statement
	// whose branches `exit 0` or `exit 1` — driven by $VERDICT so each
	// table case can steer the outcome without needing external tools
	// (jq/copilot/timeout) inside the test.
	const cgaLikeBlock = `[ "$CGA_RUNNING" = "1" ] && exit 0
export CGA_RUNNING=1
case "$VERDICT" in
  PASS)
    exit 0
    ;;
  FAIL)
    exit 1
    ;;
esac
`

	cases := []struct {
		name          string
		env           []string
		wantExit      int
		wantSecondRan bool
	}{
		{
			name:          "verdict pass: second block still runs",
			env:           []string{"VERDICT=PASS"},
			wantExit:      0,
			wantSecondRan: true,
		},
		{
			name:          "reentry guard fires: exit 0 does not dead-code second block",
			env:           []string{"VERDICT=PASS", "CGA_RUNNING=1"},
			wantExit:      0,
			wantSecondRan: true,
		},
		{
			name:          "verdict fail: hook halts, second block never runs",
			env:           []string{"VERDICT=FAIL"},
			wantExit:      1,
			wantSecondRan: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workspace, hookPath := setup(t, "pre-commit")
			markerFile := filepath.Join(workspace, "second-block-ran")

			if err := githooks.WriteBlock(workspace, "pre-commit",
				"# >>> capiko:cga:pre-commit >>>", "# <<< capiko:cga:pre-commit <<<",
				cgaLikeBlock,
			); err != nil {
				t.Fatalf("WriteBlock (cga-like): %v", err)
			}
			if err := githooks.WriteBlock(workspace, "pre-commit",
				"# >>> capiko:marker >>>", "# <<< capiko:marker <<<",
				`echo ran >> "`+markerFile+`"`,
			); err != nil {
				t.Fatalf("WriteBlock (marker): %v", err)
			}

			cmd := exec.Command("sh", hookPath)
			cmd.Env = append(os.Environ(), tc.env...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()

			gotExit := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					gotExit = exitErr.ExitCode()
				} else {
					t.Fatalf("run hook: %v\nstderr: %s", err, stderr.String())
				}
			}
			if gotExit != tc.wantExit {
				t.Errorf("hook exit code = %d, want %d\nstderr: %s", gotExit, tc.wantExit, stderr.String())
			}

			_, statErr := os.Stat(markerFile)
			secondRan := statErr == nil
			if secondRan != tc.wantSecondRan {
				t.Errorf("second block ran = %v, want %v", secondRan, tc.wantSecondRan)
			}
		})
	}
}
