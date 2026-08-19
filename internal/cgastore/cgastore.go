// Package cgastore is the shared local JSON persistence for CGA learned
// rules. It is deliberately pure — plain functions over an explicit path, no
// seams, no global state — so both cmd/capiko-ai (CLI) and internal/tui (TUI)
// can depend on the same on-disk contract without duplicating it.
package cgastore

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/martinhg/capiko-ai/internal/cga"
)

// learnedRulesFileName is the JSON file name inside a project's store
// directory (spec F3.4, design D3).
const learnedRulesFileName = "learned-rules.json"

// DefaultStorePath returns the standard local JSON store path for a
// project's learned rules: "{baseDir}/cga/{project}/learned-rules.json"
// (spec F3.4, design D3).
func DefaultStorePath(baseDir, project string) string {
	return filepath.Join(baseDir, "cga", project, learnedRulesFileName)
}

// LoadLearnedRules reads the local JSON store of learned rules at path. A
// missing file is not an error — it yields an empty slice, matching the
// store's "no rules yet" steady state.
func LoadLearnedRules(path string) ([]cga.LearnedRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []cga.LearnedRule{}, nil
		}
		return nil, err
	}
	var rules []cga.LearnedRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, err
	}
	return rules, nil
}

// SaveLearnedRules writes rules to the local JSON store at path, creating
// the containing directory if needed.
func SaveLearnedRules(path string, rules []cga.LearnedRule) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
