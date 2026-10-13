package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUpdateCommand_ShowsVersions(t *testing.T) {
	orig := fetchLatestVersions
	defer func() { fetchLatestVersions = orig }()
	fetchLatestVersions = func(_ context.Context) (string, string) {
		return "99.0.0", "4.0.0"
	}

	var buf bytes.Buffer
	handled, code, err := updateCommand("update", nil, &buf)
	if !handled {
		t.Fatal("expected handled")
	}
	if code != 0 || err != nil {
		t.Fatalf("exitCode=%d err=%v", code, err)
	}

	out := buf.String()
	if !strings.Contains(out, "capiko-ai") {
		t.Errorf("output should contain capiko-ai, got:\n%s", out)
	}
	if !strings.Contains(out, "engram") {
		t.Errorf("output should contain engram, got:\n%s", out)
	}
}

func TestUpdateCommand_WrongName(t *testing.T) {
	handled, _, _ := updateCommand("other", nil, &bytes.Buffer{})
	if handled {
		t.Error("should not handle unknown command name")
	}
}

func TestVersionLine_UpToDate(t *testing.T) {
	got := versionLine("1.0.0", "1.0.0")
	if got != "1.0.0 (up to date)" {
		t.Errorf("versionLine = %q", got)
	}
}

func TestVersionLine_UpdateAvailable(t *testing.T) {
	got := versionLine("1.0.0", "2.0.0")
	if got != "1.0.0 (latest: 2.0.0)" {
		t.Errorf("versionLine = %q", got)
	}
}

func TestVersionLine_NoLatest(t *testing.T) {
	got := versionLine("1.0.0", "")
	if got != "1.0.0" {
		t.Errorf("versionLine = %q", got)
	}
}

func TestVersionLine_Unknown(t *testing.T) {
	got := versionLine("", "1.0.0")
	if got != "unknown" {
		t.Errorf("versionLine = %q", got)
	}
}
