package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		name            string
		current, latest string
		want            bool
	}{
		{"patch bump", "0.1.0", "0.1.1", true},
		{"minor bump", "0.1.0", "0.2.0", true},
		{"major bump", "0.9.9", "1.0.0", true},
		{"equal", "1.2.3", "1.2.3", false},
		{"older latest", "1.2.3", "1.2.2", false},
		{"v prefix on both", "v1.0.0", "v1.0.1", true},
		{"v prefix on latest only", "1.0.0", "v1.1.0", true},
		{"dev current is never outdated", "0.0.0-SNAPSHOT-abc", "9.9.9", false},
		{"build metadata current skipped", "1.0.0+meta", "2.0.0", false},
		{"malformed current", "not-a-version", "1.0.0", false},
		{"malformed latest", "1.0.0", "garbage", false},
		{"latest prerelease core compared", "1.0.0", "1.1.0-rc1", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNewer(tc.current, tc.latest); got != tc.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.latest, got, tc.want)
			}
		})
	}
}

func TestReleaseURL(t *testing.T) {
	tests := []struct {
		version, want string
	}{
		{"1.4.0", "https://github.com/martinhg/capiko-ai/releases/tag/v1.4.0"},
		{"v2.0.0", "https://github.com/martinhg/capiko-ai/releases/tag/v2.0.0"},
	}
	for _, tc := range tests {
		if got := ReleaseURL(tc.version); got != tc.want {
			t.Errorf("ReleaseURL(%q) = %q, want %q", tc.version, got, tc.want)
		}
	}
}

func TestLatestParsesTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Errorf("missing User-Agent header (GitHub rejects it)")
		}
		_, _ = w.Write([]byte(`{"tag_name":"v1.4.2","name":"capiko-ai 1.4.2"}`))
	}))
	defer srv.Close()

	restore := latestURL
	latestURL = srv.URL
	defer func() { latestURL = restore }()

	got, err := Latest(context.Background())
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got != "1.4.2" {
		t.Errorf("Latest = %q, want %q (leading v stripped)", got, "1.4.2")
	}
}

func TestLatestErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"no releases yet", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) }},
		{"empty tag", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"tag_name":""}`)) }},
		{"bad json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{not json`)) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			restore := latestURL
			latestURL = srv.URL
			defer func() { latestURL = restore }()

			if _, err := Latest(context.Background()); err == nil {
				t.Errorf("expected an error, got nil")
			}
		})
	}
}
