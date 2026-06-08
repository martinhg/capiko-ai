package doctor

import (
	"encoding/json"
	"strings"
	"testing"
)

func sampleReport() Report {
	return Report{Checks: []Check{
		{Name: "Operating system", Status: Pass, Detail: "darwin/arm64 supported"},
		{Name: "Copilot CLI", Status: Fail, Detail: "not found on PATH", Remedy: "install copilot from https://example"},
		{Name: "Skill drift", Status: Warn, Detail: "1 skill(s) differ", Remedy: "run Sync"},
	}}
}

func TestRenderTextShowsEveryCheckAndRemedies(t *testing.T) {
	out := RenderText(sampleReport())

	for _, want := range []string{"Operating system", "Copilot CLI", "Skill drift"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing check %q\n%s", want, out)
		}
	}
	// Remedies must surface for non-pass checks so the user knows what to do.
	if !strings.Contains(out, "install copilot from https://example") {
		t.Errorf("text output missing the Copilot remedy\n%s", out)
	}
	if !strings.Contains(out, "run Sync") {
		t.Errorf("text output missing the drift remedy\n%s", out)
	}
	// A summary line with the tally.
	if !strings.Contains(out, "1 pass") || !strings.Contains(out, "1 warn") || !strings.Contains(out, "1 fail") {
		t.Errorf("text output missing the summary tally\n%s", out)
	}
}

func TestRenderTextPassChecksHideRemedy(t *testing.T) {
	out := RenderText(Report{Checks: []Check{{Name: "OK check", Status: Pass, Detail: "all good"}}})
	if strings.Contains(out, "remedy") || strings.Contains(out, "→") {
		t.Errorf("a passing check should not print a remedy arrow\n%s", out)
	}
}

func TestRenderJSONEmitsStringStatuses(t *testing.T) {
	out, err := RenderJSON(sampleReport())
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	// Status must serialize as a string, not the raw int, so consumers can read it.
	if !strings.Contains(out, `"status": "fail"`) {
		t.Errorf("JSON should emit string statuses (\"fail\")\n%s", out)
	}

	// And it must round-trip as valid JSON.
	var parsed struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("RenderJSON produced invalid JSON: %v\n%s", err, out)
	}
	if len(parsed.Checks) != 3 {
		t.Fatalf("want 3 checks in JSON, got %d", len(parsed.Checks))
	}
	if parsed.Checks[1].Status != "fail" {
		t.Errorf("Copilot CLI status: want \"fail\", got %q", parsed.Checks[1].Status)
	}
}
