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
