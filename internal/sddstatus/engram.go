package sddstatus

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// artifactStoreRe matches an artifact_store or artifactStore YAML key and captures
// the value. Case-insensitive multiline to handle either key variant.
var artifactStoreRe = regexp.MustCompile(`(?mi)^\s*artifact[_]?store\s*:\s*["']?([A-Za-z]+)`)

// configArtifactStoreIsEngram reports whether the openspec config file declares
// artifact_store (or artifactStore) as "engram" or "hybrid". It reads
// openspec/config.yaml first, then openspec/config.yml. No YAML dependency — a
// narrow regex over the raw text is sufficient.
func configArtifactStoreIsEngram(cwd string) bool {
	for _, name := range []string{"config.yaml", "config.yml"} {
		p := filepath.Join(cwd, "openspec", name)
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		m := artifactStoreRe.FindSubmatch(raw)
		if m == nil {
			continue
		}
		val := strings.ToLower(string(m[1]))
		if val == "engram" || val == "hybrid" {
			return true
		}
	}
	return false
}

// shouldTryEngram reports whether the Engram fallback is enabled for the given
// workspace. Any one of the three triggers is independently sufficient:
//
//   - CAPIKO_SDD_STATUS_ENGRAM environment variable is set (any non-empty value)
//   - A .engram/ directory exists at <cwd>/.engram
//   - openspec/config.yaml or openspec/config.yml declares artifact_store: engram|hybrid
func shouldTryEngram(cwd string) bool {
	if os.Getenv("CAPIKO_SDD_STATUS_ENGRAM") != "" {
		return true
	}
	if info, err := os.Stat(filepath.Join(cwd, ".engram")); err == nil && info.IsDir() {
		return true
	}
	return configArtifactStoreIsEngram(cwd)
}

// ---------------------------------------------------------------------------
// Engram export seam
// ---------------------------------------------------------------------------

// engramObservation is a single record from `engram export`. Only the four
// fields the Engram fallback path needs are captured; additional JSON fields
// are silently ignored.
type engramObservation struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Project string `json:"project"`
	Scope   string `json:"scope"`
}

// engramExport is a test seam. Tests swap it to return canned observations;
// the real implementation shells out to the engram binary. Tests NEVER shell
// out to the real binary.
var engramExport = exportEngramObservations

// exportEngramObservations is the real implementation behind the seam. It
// creates a temp file, runs `engram export <path>`, reads the JSON result,
// and returns the parsed observations. Any failure returns a non-nil error so
// the caller can degrade gracefully.
func exportEngramObservations() ([]engramObservation, error) {
	tmp, err := os.CreateTemp("", "capiko-engram-export-*.json")
	if err != nil {
		return nil, err
	}
	path := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(path)

	if out, err := exec.Command("engram", "export", path).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("engram export: %w: %s", err, strings.TrimSpace(string(out)))
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Observations []engramObservation `json:"observations"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc.Observations, nil
}

// ---------------------------------------------------------------------------
// Project inference
// ---------------------------------------------------------------------------

// gitOriginURLRe finds the `url` field inside a [remote "origin"] section in
// a git config file. The `[^\[]*?` part matches across lines up to (but not
// including) the next section header.
var gitOriginURLRe = regexp.MustCompile(`(?m)^\[remote "origin"\][^\[]*?url\s*=\s*(\S+)`)

// gitOwnerRepoRe extracts the owner/repo slug from a git remote URL, stripping
// a trailing .git suffix. Works for both HTTPS and SSH URLs.
var gitOwnerRepoRe = regexp.MustCompile(`[:/]([^/:]+/[^/]+?)(?:\.git)?$`)

// inferEngramProject returns the project identifier for the Engram matching
// step, using the first match from this chain:
//  1. ENGRAM_PROJECT environment variable (if non-empty)
//  2. owner/repo from [remote "origin"] in <cwd>/.git/config (lowercased)
//  3. Lowercased basename of cwd
func inferEngramProject(cwd string) string {
	if p := strings.TrimSpace(os.Getenv("ENGRAM_PROJECT")); p != "" {
		return p
	}
	if p := projectFromGitConfig(cwd); p != "" {
		return p
	}
	return strings.ToLower(filepath.Base(cwd))
}

// projectFromGitConfig reads <cwd>/.git/config and extracts the lowercased
// owner/repo from the [remote "origin"] url field. Returns "" when the file
// is absent, the section is missing, or the URL does not match the expected
// shape — the caller falls back to the directory basename.
func projectFromGitConfig(cwd string) string {
	raw, err := os.ReadFile(filepath.Join(cwd, ".git", "config"))
	if err != nil {
		return ""
	}
	m := gitOriginURLRe.FindSubmatch(raw)
	if m == nil {
		return ""
	}
	m2 := gitOwnerRepoRe.FindSubmatch(m[1])
	if m2 == nil {
		return ""
	}
	return strings.ToLower(string(m2[1]))
}

// ---------------------------------------------------------------------------
// Observation helpers
// ---------------------------------------------------------------------------

// titleRe parses an observation title of the form sdd/<change>/<artifactType>
// and captures the change name and artifact type in groups 1 and 2.
var titleRe = regexp.MustCompile(`^sdd/([^/]+)/(proposal|spec|design|tasks|apply-progress|verify-report|state)$`)

// engramObservationMatchesProject reports whether obs belongs to the given
// project and is not personal-scope. Comparison is case-insensitive.
func engramObservationMatchesProject(obs engramObservation, project string) bool {
	return strings.EqualFold(obs.Project, project) && !strings.EqualFold(obs.Scope, "personal")
}

// collectEngramChanges returns a sorted, deduplicated slice of change names
// found in the project-matching, non-personal observations.
func collectEngramChanges(obs []engramObservation, project string) []string {
	seen := map[string]bool{}
	for _, o := range obs {
		if !engramObservationMatchesProject(o, project) {
			continue
		}
		m := titleRe.FindStringSubmatch(o.Title)
		if m == nil {
			continue
		}
		seen[m[1]] = true
	}
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// selectEngramChange selects exactly one change from the list:
//   - requested != "": returns it iff present in changes, else ("", false).
//   - requested == "": returns the single entry when len == 1, else ("", false).
//
// This is fail-safe: multiple Engram changes with no explicit request are not
// auto-selected — the caller returns the normal sdd-new blocked status.
func selectEngramChange(changes []string, requested string) (string, bool) {
	if requested != "" {
		for _, c := range changes {
			if c == requested {
				return c, true
			}
		}
		return "", false
	}
	if len(changes) == 1 {
		return changes[0], true
	}
	return "", false
}

// engramArtifactLookup returns the content and a present flag for the
// observation matching sdd/<change>/<artifactType> in the given project.
func engramArtifactLookup(obs []engramObservation, change, project, artifactType string) (content string, present bool) {
	want := "sdd/" + change + "/" + artifactType
	for _, o := range obs {
		if o.Title == want && engramObservationMatchesProject(o, project) {
			return o.Content, true
		}
	}
	return "", false
}

// engramArtifactContent returns the content of the matching observation, or
// "" when no matching observation exists.
func engramArtifactContent(obs []engramObservation, change, project, artifactType string) string {
	content, _ := engramArtifactLookup(obs, change, project, artifactType)
	return content
}

// engramArtifactState is a content-based twin of singleArtifactState for use
// when the artifact source is an Engram observation rather than a file.
func engramArtifactState(content string, present bool) ArtifactState {
	if !present {
		return ArtifactMissing
	}
	if strings.TrimSpace(content) != "" {
		return ArtifactDone
	}
	return ArtifactPartial
}

// engramArtifactsForChange builds the same six-key artifact map that the
// file-path produces, from the Engram observations for a change.
func engramArtifactsForChange(obs []engramObservation, change, project string) map[string]ArtifactState {
	type entry struct{ titleType, key string }
	entries := []entry{
		{"proposal", "proposal"},
		{"spec", "specs"},
		{"design", "design"},
		{"tasks", "tasks"},
		{"apply-progress", "applyProgress"},
		{"verify-report", "verifyReport"},
	}
	artifacts := make(map[string]ArtifactState, len(entries))
	for _, e := range entries {
		content, present := engramArtifactLookup(obs, change, project, e.titleType)
		artifacts[e.key] = engramArtifactState(content, present)
	}
	return artifacts
}

// engramArtifactPaths returns an ArtifactPaths with sentinel
// "engram:sdd/<change>/<stem>" paths for non-missing artifacts, and empty
// slices for missing ones. The engram: prefix signals consumers that these
// are not filesystem paths.
func engramArtifactPaths(change string, artifacts map[string]ArtifactState) ArtifactPaths {
	sentinel := func(key, stem string) []string {
		if artifacts[key] == ArtifactMissing {
			return nil
		}
		return []string{"engram:sdd/" + change + "/" + stem}
	}
	return ArtifactPaths{
		Proposal:      sentinel("proposal", "proposal"),
		Specs:         sentinel("specs", "spec"),
		Design:        sentinel("design", "design"),
		Tasks:         sentinel("tasks", "tasks"),
		ApplyProgress: sentinel("applyProgress", "apply-progress"),
		VerifyReport:  sentinel("verifyReport", "verify-report"),
	}.withArrays()
}

// ---------------------------------------------------------------------------
// Engram fallback resolver
// ---------------------------------------------------------------------------

// resolveEngramStatus reconstructs a capiko.sdd-status from Engram observations
// when the file path found no matching change. It never errors: any failure
// (gating off, export error, malformed JSON, no matching change) degrades to
// ok=false so Resolve returns the normal blocked status. Files stay canonical.
//
// The one exception: when multiple Engram changes exist and no change is
// requested, the caller gets a select-change blocked status (ok=true) so the
// user sees the Engram-aware ambiguity message rather than the generic sdd-new.
func resolveEngramStatus(cwd, requested string) (Status, bool) {
	if !shouldTryEngram(cwd) {
		return Status{}, false
	}
	obs, err := engramExport()
	if err != nil {
		return Status{}, false // binary absent / non-zero exit / malformed JSON
	}
	project := inferEngramProject(cwd)

	changes := collectEngramChanges(obs, project)

	// Ambiguity: multiple Engram changes with no explicit request → surface a
	// select-change blocked status so the user sees the names rather than a
	// generic sdd-new. ok=true causes Resolve to use this status instead.
	if len(changes) > 1 && requested == "" {
		joined := strings.Join(changes, ", ")
		reasons := []string{"Multiple Engram SDD changes found. Specify which to resume: " + joined + "."}
		return blockedStatus(cwd, nil, "select-change", reasons), true
	}

	change, ok := selectEngramChange(changes, requested)
	if !ok {
		return Status{}, false // zero changes, or requested not found in Engram
	}

	artifacts := engramArtifactsForChange(obs, change, project)
	tasksText := engramArtifactContent(obs, change, project, "tasks")
	verifyText := engramArtifactContent(obs, change, project, "verify-report")

	taskProgress := countTaskProgressText(tasksText)
	verifyPassing := reportTextIsClearlyPassing(verifyText)

	coreReady := artifacts["proposal"] == ArtifactDone &&
		artifacts["specs"] == ArtifactDone &&
		artifacts["design"] == ArtifactDone &&
		artifacts["tasks"] == ArtifactDone &&
		taskProgress.Total > 0
	applyState := resolveApplyState(coreReady, taskProgress)

	blockedReasons := artifactBlockedReasons(artifacts, taskProgress)
	if artifacts["verifyReport"] == ArtifactDone && !verifyPassing && applyState != ApplyReady {
		blockedReasons = append(blockedReasons, "verify-report.md is not clearly passing.")
	}
	dependencies := resolveDependencies(artifacts, taskProgress, applyState, coreReady, verifyPassing)
	nextRecommended := resolveNextRecommended(dependencies, applyState)

	root := "engram:sdd/" + change
	status := baseStatus(cwd, &change, &root, nextRecommended, blockedReasons)
	status.ArtifactStore = ArtifactStoreEngram
	status.PlanningHome.Path = "engram:sdd"
	status.ArtifactPaths = engramArtifactPaths(change, artifacts)
	status.Artifacts = artifacts
	status.TaskProgress = taskProgress
	status.Dependencies = dependencies
	status.ApplyState = applyState
	return status, true
}
