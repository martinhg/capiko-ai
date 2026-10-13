package engram

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDetected(t *testing.T) {
	// Detected returns true when "engram" is findable and false when it is not.
	// We cannot control PATH reliably in a unit test, but we can at least verify
	// the function does not panic.
	_ = Detected()
}

func TestLatestVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"tag_name":"v3.5.0"}`)
	}))
	defer srv.Close()

	orig := latestURL
	latestURL = srv.URL
	defer func() { latestURL = orig }()

	v, err := LatestVersion(context.Background())
	if err != nil {
		t.Fatalf("LatestVersion: %v", err)
	}
	if v != "3.5.0" {
		t.Errorf("LatestVersion = %q, want 3.5.0", v)
	}
}

func TestLatestVersion_EmptyTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"tag_name":""}`)
	}))
	defer srv.Close()

	orig := latestURL
	latestURL = srv.URL
	defer func() { latestURL = orig }()

	_, err := LatestVersion(context.Background())
	if err == nil {
		t.Error("expected error for empty tag_name")
	}
}

func TestLatestVersion_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := latestURL
	latestURL = srv.URL
	defer func() { latestURL = orig }()

	_, err := LatestVersion(context.Background())
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
}

func TestGoAvailable(t *testing.T) {
	if !GoAvailable() {
		t.Skip("go not on PATH in this environment")
	}
}

func TestUpgrade_Success(t *testing.T) {
	orig := runGoInstall
	defer func() { runGoInstall = orig }()

	var gotPkg string
	runGoInstall = func(_ context.Context, pkg string) (string, error) {
		gotPkg = pkg
		return "", nil
	}

	if err := Upgrade(context.Background(), "3.5.0"); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	want := engramModule + "@v3.5.0"
	if gotPkg != want {
		t.Errorf("go install pkg = %q, want %q", gotPkg, want)
	}
}

func TestUpgrade_Latest(t *testing.T) {
	orig := runGoInstall
	defer func() { runGoInstall = orig }()

	var gotPkg string
	runGoInstall = func(_ context.Context, pkg string) (string, error) {
		gotPkg = pkg
		return "", nil
	}

	if err := Upgrade(context.Background(), ""); err != nil {
		t.Fatalf("Upgrade: %v", err)
	}
	want := engramModule + "@latest"
	if gotPkg != want {
		t.Errorf("go install pkg = %q, want %q", gotPkg, want)
	}
}

func TestUpgrade_GoInstallFails(t *testing.T) {
	orig := runGoInstall
	defer func() { runGoInstall = orig }()

	runGoInstall = func(_ context.Context, _ string) (string, error) {
		return "module not found", fmt.Errorf("exit status 1")
	}

	err := Upgrade(context.Background(), "99.0.0")
	if err == nil {
		t.Fatal("expected error when go install fails")
	}
	if got := err.Error(); got == "" {
		t.Error("error message should not be empty")
	}
}

func TestLatestVersion_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	orig := latestURL
	origClient := httpClient
	latestURL = srv.URL
	httpClient = &http.Client{Timeout: 50 * time.Millisecond}
	defer func() { latestURL = orig; httpClient = origClient }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := LatestVersion(ctx)
	if err == nil {
		t.Error("expected timeout error")
	}
}
