package catalog

import "testing"

func TestLoadEmbedded(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("embedded catalog is empty")
	}

	byName := map[string]bool{}
	for _, s := range got {
		if s.Content == "" {
			t.Errorf("skill %q has empty content", s.Name)
		}
		if s.Description == "" {
			t.Errorf("skill %q has empty description", s.Name)
		}
		byName[s.Name] = true
	}

	if !byName["capiko-hello"] {
		t.Errorf("expected capiko-hello in catalog, got %v", byName)
	}

	// The SDD phase skills bundle must be present and parse.
	for _, phase := range []string{"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive"} {
		if !byName["sdd-"+phase] {
			t.Errorf("expected sdd-%s in catalog", phase)
		}
	}
}
