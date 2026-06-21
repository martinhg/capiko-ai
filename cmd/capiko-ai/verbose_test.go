package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/clilog"
)

func TestParseVerboseStripsFlag(t *testing.T) {
	rest, verbose := parseVerbose([]string{"--json", "--verbose", "--repair"})
	if !verbose {
		t.Error("--verbose should be detected")
	}
	if strings.Join(rest, ",") != "--json,--repair" {
		t.Errorf("rest = %v, want [--json --repair] without --verbose", rest)
	}

	rest, verbose = parseVerbose([]string{"--json"})
	if verbose {
		t.Error("--verbose should not be detected when absent")
	}
	if len(rest) != 1 {
		t.Errorf("rest = %v, want unchanged", rest)
	}
}

// stubLogSink captures --verbose output for assertions.
func stubLogSink(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := verboseLogSink
	verboseLogSink = &buf
	t.Cleanup(func() { verboseLogSink = prev })
	return &buf
}

func TestVerboseEmitsJSONLinesToSinkNotStdout(t *testing.T) {
	withStubInputs(t, healthyInputs())
	logBuf := stubLogSink(t)

	var out bytes.Buffer
	if _, _, err := doctorCommand("doctor", []string{"--verbose"}, &out); err != nil {
		t.Fatalf("doctorCommand: %v", err)
	}

	logged := strings.TrimSpace(logBuf.String())
	if logged == "" {
		t.Fatal("--verbose should emit log lines to the sink")
	}
	// Every emitted line must be a valid clilog JSON entry tagged with the command.
	for _, line := range strings.Split(logged, "\n") {
		var e clilog.Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("log line is not valid JSON: %v\n%s", err, line)
		}
		if e.Command != "doctor" {
			t.Errorf("entry command = %q, want doctor", e.Command)
		}
	}
	// stdout stays clean: the human report, never the JSON log lines.
	if strings.Contains(out.String(), `"command":"doctor"`) {
		t.Errorf("verbose JSON must go to the sink, not stdout:\n%s", out.String())
	}
}

func TestNoVerboseWritesNothingToSink(t *testing.T) {
	withStubInputs(t, healthyInputs())
	logBuf := stubLogSink(t)

	var out bytes.Buffer
	if _, _, err := doctorCommand("doctor", nil, &out); err != nil {
		t.Fatalf("doctorCommand: %v", err)
	}
	if logBuf.Len() != 0 {
		t.Errorf("without --verbose the sink must stay empty, got:\n%s", logBuf.String())
	}
}
