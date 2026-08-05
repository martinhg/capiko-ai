package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestUsage_ContainsAllCommands(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	out := buf.String()
	for name := range commands {
		if !strings.Contains(out, name) {
			t.Errorf("usage output missing command %q", name)
		}
	}
}

func TestUsage_ContainsHeader(t *testing.T) {
	var buf bytes.Buffer
	usage(&buf)
	if !strings.Contains(buf.String(), "capiko-ai") {
		t.Error("usage should mention capiko-ai")
	}
}

func TestSuggest_PrefixMatch(t *testing.T) {
	if got := suggest("doc"); got != "doctor" {
		t.Errorf("suggest(doc) = %q, want doctor", got)
	}
}

func TestSuggest_SubstringMatch(t *testing.T) {
	if got := suggest("registry"); got != "skill-registry" {
		t.Errorf("suggest(registry) = %q, want skill-registry", got)
	}
}

func TestSuggest_NoMatch(t *testing.T) {
	if got := suggest("xyzzy"); got != "" {
		t.Errorf("suggest(xyzzy) = %q, want empty", got)
	}
}

func TestSuggest_CaseInsensitive(t *testing.T) {
	if got := suggest("DOC"); got != "doctor" {
		t.Errorf("suggest(DOC) = %q, want doctor", got)
	}
}

func TestIsTerminal_RegularFile(t *testing.T) {
	// A temp file is NOT a terminal.
	f, err := os.CreateTemp(t.TempDir(), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("a regular file should not be a terminal")
	}
}
