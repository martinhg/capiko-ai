package rdd

import "encoding/json"

// findingsIssues is the minimal shape rdd needs to extract paths from a
// lens result's opaque findings JSON (spec bounded-correction; design
// "Interfaces / Contracts": `{"issues": [{"path": "...", ...}]}`). rdd does
// not otherwise interpret findings — this is a narrow, read-only path
// extraction, not a full findings schema.
type findingsIssues struct {
	Issues []struct {
		Path string `json:"path"`
	} `json:"issues"`
}

// PathScopeCheck reports whether every path in correctionPaths is present
// in findingPaths (spec bounded-correction "Path-scope admission": a
// correction MUST be admitted only if every path it touches is a subset of
// the paths reported in the frozen findings). An empty correctionPaths
// always returns true — a no-change correction has nothing to be
// out-of-scope (design "Interfaces / Contracts").
func PathScopeCheck(correctionPaths, findingPaths []string) bool {
	if len(correctionPaths) == 0 {
		return true
	}
	allowed := make(map[string]bool, len(findingPaths))
	for _, p := range findingPaths {
		allowed[p] = true
	}
	for _, p := range correctionPaths {
		if !allowed[p] {
			return false
		}
	}
	return true
}

// ExtractFindingPaths parses findings as `{"issues": [{"path": "...",
// ...}]}` (design "Interfaces / Contracts") and returns the deduplicated,
// non-empty paths across all issues, in first-seen order. Malformed JSON or
// a missing/empty "issues" key returns nil, signaling the caller to fall
// back to the frozen candidate's changed paths (design "Path fallback when
// findings lack path fields").
func ExtractFindingPaths(findings json.RawMessage) []string {
	if len(findings) == 0 {
		return nil
	}

	var parsed findingsIssues
	if err := json.Unmarshal(findings, &parsed); err != nil {
		return nil
	}

	var paths []string
	seen := make(map[string]bool, len(parsed.Issues))
	for _, issue := range parsed.Issues {
		if issue.Path == "" || seen[issue.Path] {
			continue
		}
		seen[issue.Path] = true
		paths = append(paths, issue.Path)
	}
	return paths
}
