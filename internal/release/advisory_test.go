package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdvisoryParsesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"body":"engram 1.17 recommended for best results"}`))
	}))
	defer srv.Close()

	orig := advisoryURL
	advisoryURL = srv.URL
	defer func() { advisoryURL = orig }()

	got := Advisory(context.Background())
	if got != "engram 1.17 recommended for best results" {
		t.Errorf("Advisory = %q, want advisory text", got)
	}
}

func TestAdvisoryEmptyOnMissingTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	orig := advisoryURL
	advisoryURL = srv.URL
	defer func() { advisoryURL = orig }()

	if got := Advisory(context.Background()); got != "" {
		t.Errorf("Advisory = %q, want empty on 404", got)
	}
}

func TestAdvisoryEmptyOnEmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"body":""}`))
	}))
	defer srv.Close()

	orig := advisoryURL
	advisoryURL = srv.URL
	defer func() { advisoryURL = orig }()

	if got := Advisory(context.Background()); got != "" {
		t.Errorf("Advisory = %q, want empty", got)
	}
}

func TestAdvisoryEmptyOnBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	defer srv.Close()

	orig := advisoryURL
	advisoryURL = srv.URL
	defer func() { advisoryURL = orig }()

	if got := Advisory(context.Background()); got != "" {
		t.Errorf("Advisory = %q, want empty on bad JSON", got)
	}
}

func TestSanitizeAdvisory(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"plain text", "hello world", "hello world"},
		{"strips ANSI", "\x1b[31mred text\x1b[0m", "red text"},
		{"strips control chars", "hello\x00\x01world", "helloworld"},
		{"first line only", "line one\nline two", "line one"},
		{"trims whitespace", "  hello  ", "hello"},
		{"CR line break", "first\r\nsecond", "first"},
		{"empty string", "", ""},
		{"only whitespace", "   \t  ", ""},
		{"caps at 200", strings.Repeat("x", 250), strings.Repeat("x", 200) + "…"},
		{"preserves tabs", "hello\tworld", "hello\tworld"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := SanitizeAdvisory(tc.input); got != tc.want {
				t.Errorf("SanitizeAdvisory = %q, want %q", got, tc.want)
			}
		})
	}
}
