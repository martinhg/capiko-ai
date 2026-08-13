package main

import (
	"errors"
	"testing"

	"github.com/martinhg/capiko-ai/internal/copilot"
)

func TestRequireHostSuccess(t *testing.T) {
	want := &copilot.Host{BinPath: "/b/copilot", ConfigDir: "/h/.copilot"}
	detect := func() (*copilot.Host, error) { return want, nil }

	host, exitCode, err := requireHost(detect)
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if host != want {
		t.Errorf("host = %v, want %v", host, want)
	}
}

func TestRequireHostCopilotNotFound(t *testing.T) {
	detect := func() (*copilot.Host, error) { return nil, nil }

	host, exitCode, err := requireHost(detect)
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", exitCode)
	}
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if host != nil {
		t.Errorf("host = %v, want nil", host)
	}
}

func TestRequireHostDetectError(t *testing.T) {
	wantErr := errors.New("no home dir")
	detect := func() (*copilot.Host, error) { return nil, wantErr }

	host, exitCode, err := requireHost(detect)
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
	if err != wantErr {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if host != nil {
		t.Errorf("host = %v, want nil", host)
	}
}
