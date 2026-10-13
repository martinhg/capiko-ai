package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/engram"
)

func TestUpgradeCommand_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v99.0.0"}`)
	}))
	defer srv.Close()

	origURL := engram.ExportLatestURL()
	origGoInstall := engram.ExportRunGoInstall()
	origRunOut := engram.ExportRunOut()
	defer func() {
		engram.SetLatestURL(origURL)
		engram.SetRunGoInstall(origGoInstall)
		engram.SetRunOut(origRunOut)
	}()

	engram.SetLatestURL(srv.URL)
	engram.SetRunOut(func(args ...string) (string, error) { return "engram 3.4.0", nil })
	engram.SetRunGoInstall(func(_ context.Context, _ string) (string, error) { return "", nil })

	var buf bytes.Buffer
	handled, code, err := upgradeCommand("upgrade", nil, &buf)
	if !handled {
		t.Fatal("expected handled")
	}
	if code != 0 || err != nil {
		t.Fatalf("exitCode=%d err=%v", code, err)
	}
	out := buf.String()
	if !strings.Contains(out, "upgrading to 99.0.0") {
		t.Errorf("should show upgrade target, got:\n%s", out)
	}
	if !strings.Contains(out, "✓") {
		t.Errorf("should contain success marker, got:\n%s", out)
	}
}

func TestUpgradeCommand_AlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v3.4.0"}`)
	}))
	defer srv.Close()

	origURL := engram.ExportLatestURL()
	origRunOut := engram.ExportRunOut()
	defer func() {
		engram.SetLatestURL(origURL)
		engram.SetRunOut(origRunOut)
	}()

	engram.SetLatestURL(srv.URL)
	engram.SetRunOut(func(args ...string) (string, error) { return "engram 3.4.0", nil })

	var buf bytes.Buffer
	handled, code, _ := upgradeCommand("upgrade", nil, &buf)
	if !handled {
		t.Fatal("expected handled")
	}
	if code != 0 {
		t.Fatalf("exitCode=%d, want 0", code)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("should say up to date, got:\n%s", buf.String())
	}
}

func TestUpgradeCommand_WrongName(t *testing.T) {
	handled, _, _ := upgradeCommand("other", nil, &bytes.Buffer{})
	if handled {
		t.Error("should not handle unknown command name")
	}
}
