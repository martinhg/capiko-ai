package sddstatus

import (
	"os"
	"regexp"
	"strings"
)

// ArtifactState is whether an artifact is absent, present-but-empty, or complete.
type ArtifactState string

const (
	ArtifactMissing ArtifactState = "missing"
	ArtifactPartial ArtifactState = "partial"
	ArtifactDone    ArtifactState = "done"
)

// DependencyState is whether a phase's prerequisites are met.
type DependencyState string

const (
	DependencyBlocked DependencyState = "blocked"
	DependencyReady   DependencyState = "ready"
	DependencyAllDone DependencyState = "all_done"
)

// ApplyState summarizes whether implementation can or did happen.
type ApplyState string

const (
	ApplyBlocked ApplyState = "blocked"
	ApplyReady   ApplyState = "ready"
	ApplyAllDone ApplyState = "all_done"
)

// Phase is an SDD phase name used for routing.
type Phase string

const (
	PhasePropose Phase = "propose"
	PhaseSpec    Phase = "spec"
	PhaseDesign  Phase = "design"
	PhaseTasks   Phase = "tasks"
	PhaseApply   Phase = "apply"
	PhaseVerify  Phase = "verify"
	PhaseArchive Phase = "archive"
)

// TaskProgress counts the checkbox tasks in tasks.md.
type TaskProgress struct {
	Total       int  `json:"total"`
	Completed   int  `json:"completed"`
	Pending     int  `json:"pending"`
	AllComplete bool `json:"allComplete"`
}

// Dependencies is the readiness of each phase.
type Dependencies struct {
	Proposal DependencyState `json:"proposal"`
	Specs    DependencyState `json:"specs"`
	Design   DependencyState `json:"design"`
	Tasks    DependencyState `json:"tasks"`
	Apply    DependencyState `json:"apply"`
	Verify   DependencyState `json:"verify"`
	Archive  DependencyState `json:"archive"`
}

// Schema identity emitted in every status payload.
const (
	SchemaName    = "capiko.sdd-status"
	SchemaVersion = 1
	artifactStore = "openspec"
	modeRepoLocal = "repo-local"
)

// PlanningHome locates the OpenSpec store.
type PlanningHome struct {
	Mode string `json:"mode"`
	Path string `json:"path"`
}

// ActionContext bounds where a phase may edit.
type ActionContext struct {
	Mode             string   `json:"mode"`
	WorkspaceRoot    string   `json:"workspaceRoot"`
	AllowedEditRoots []string `json:"allowedEditRoots"`
}

// Status is the computed state of an SDD change. Its JSON form is the
// capiko.sdd-status contract consumed by the SDD skills.
type Status struct {
	SchemaName      string                   `json:"schemaName"`
	SchemaVersion   int                      `json:"schemaVersion"`
	ChangeName      *string                  `json:"changeName"`
	ArtifactStore   string                   `json:"artifactStore"`
	PlanningHome    PlanningHome             `json:"planningHome"`
	ChangeRoot      *string                  `json:"changeRoot"`
	ArtifactPaths   ArtifactPaths            `json:"artifactPaths"`
	Artifacts       map[string]ArtifactState `json:"artifacts"`
	TaskProgress    TaskProgress             `json:"taskProgress"`
	Dependencies    Dependencies             `json:"dependencies"`
	ApplyState      ApplyState               `json:"applyState"`
	ActionContext   ActionContext            `json:"actionContext"`
	NextRecommended string                   `json:"nextRecommended"`
	BlockedReasons  []string                 `json:"blockedReasons"`
}

// ResolveOptions selects the workspace and (optionally) the change to resolve.
type ResolveOptions struct {
	Cwd        string // workspace root; "" means the current directory
	ChangeName string // explicit change; "" infers from active changes
}

// Resolve computes the SDD status of a change from its OpenSpec artifacts. It
// never errors on a missing or ambiguous change — those return a blocked status
// with a routing token (sdd-new / select-change) and reasons.
func Resolve(options ResolveOptions) (Status, error) {
	cwd := options.Cwd
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Status{}, err
		}
		cwd = wd
	}

	changeName, blocked, reasons, err := selectChange(cwd, options.ChangeName)
	if err != nil {
		return Status{}, err
	}
	if blocked != "" {
		return blockedStatus(cwd, changeNamePtr(changeName), blocked, reasons), nil
	}

	root := changeRoot(cwd, changeName)
	paths := ResolveArtifactPaths(cwd, changeName)
	artifacts := map[string]ArtifactState{
		"proposal":      singleArtifactState(paths.Proposal),
		"specs":         singleArtifactState(paths.Specs),
		"design":        singleArtifactState(paths.Design),
		"tasks":         singleArtifactState(paths.Tasks),
		"applyProgress": singleArtifactState(paths.ApplyProgress),
		"verifyReport":  singleArtifactState(paths.VerifyReport),
	}
	taskProgress := countTaskProgress(firstPath(paths.Tasks))
	verifyReportPassing := reportIsClearlyPassing(firstPath(paths.VerifyReport))

	coreReady := artifacts["proposal"] == ArtifactDone &&
		artifacts["specs"] == ArtifactDone &&
		artifacts["design"] == ArtifactDone &&
		artifacts["tasks"] == ArtifactDone &&
		taskProgress.Total > 0
	applyState := resolveApplyState(coreReady, taskProgress)

	blockedReasons := artifactBlockedReasons(artifacts, taskProgress)
	if artifacts["verifyReport"] == ArtifactDone && !verifyReportPassing && applyState != ApplyReady {
		blockedReasons = append(blockedReasons, "verify-report.md is not clearly passing.")
	}
	dependencies := resolveDependencies(artifacts, taskProgress, applyState, coreReady, verifyReportPassing)
	nextRecommended := resolveNextRecommended(dependencies, applyState)

	status := baseStatus(cwd, changeNamePtr(changeName), &root, nextRecommended, blockedReasons)
	status.ArtifactPaths = paths.withArrays()
	status.Artifacts = artifacts
	status.TaskProgress = taskProgress
	status.Dependencies = dependencies
	status.ApplyState = applyState
	return status, nil
}

// selectChange resolves which change to act on. It returns a routing token and
// reasons when selection is blocked (no name and zero/ambiguous active changes,
// or a named change that does not exist).
func selectChange(cwd, requested string) (name, blocked string, reasons []string, err error) {
	active, err := ListActiveOpenSpecChanges(cwd)
	if err != nil {
		return "", "", nil, err
	}
	if requested != "" {
		for _, c := range active {
			if c == requested {
				return requested, "", nil, nil
			}
		}
		return requested, "sdd-new", []string{"Active OpenSpec change not found: " + requested + "."}, nil
	}
	switch len(active) {
	case 0:
		return "", "sdd-new", []string{"No active OpenSpec changes found under openspec/changes."}, nil
	case 1:
		return active[0], "", nil, nil
	default:
		return "", "select-change", []string{"Change selection is ambiguous: " + strings.Join(active, ", ") + "."}, nil
	}
}

func resolveApplyState(coreReady bool, tp TaskProgress) ApplyState {
	switch {
	case !coreReady:
		return ApplyBlocked
	case tp.AllComplete:
		return ApplyAllDone
	default:
		return ApplyReady
	}
}

func resolveDependencies(artifacts map[string]ArtifactState, tp TaskProgress, applyState ApplyState, coreReady, verifyPassing bool) Dependencies {
	// Planning phases follow the DAG proposal → spec/design → tasks. A phase is
	// "ready" when its prerequisites are done but its own artifact is not, so the
	// engine can route the next planning step deterministically instead of
	// emitting a generic "resolve-blockers".
	proposalDone := artifacts["proposal"] == ArtifactDone
	specDone := artifacts["specs"] == ArtifactDone
	designDone := artifacts["design"] == ArtifactDone
	d := Dependencies{
		Proposal: planningDependency(true, artifacts["proposal"]),
		Specs:    planningDependency(proposalDone, artifacts["specs"]),
		Design:   planningDependency(proposalDone, artifacts["design"]),
		Tasks:    planningDependency(specDone && designDone, artifacts["tasks"]),
		Apply:    DependencyBlocked,
		Verify:   DependencyBlocked,
		Archive:  DependencyBlocked,
	}
	switch applyState {
	case ApplyReady:
		d.Apply = DependencyReady
	case ApplyAllDone:
		d.Apply = DependencyAllDone
	}

	applyProgressDone := artifacts["applyProgress"] == ArtifactDone
	verifyReportDone := artifacts["verifyReport"] == ArtifactDone
	switch {
	case verifyReportDone && coreReady && tp.AllComplete && verifyPassing:
		d.Verify = DependencyAllDone
	case coreReady && (applyState == ApplyAllDone || applyProgressDone):
		d.Verify = DependencyReady
	}
	if d.Verify == DependencyAllDone && tp.AllComplete {
		d.Archive = DependencyReady
	}
	return d
}

// planningDependency reports a planning phase's readiness: all_done when its
// own artifact is complete, ready when its prerequisites are met but the
// artifact is not yet complete, blocked otherwise.
func planningDependency(prereqsDone bool, own ArtifactState) DependencyState {
	switch {
	case own == ArtifactDone:
		return DependencyAllDone
	case prereqsDone:
		return DependencyReady
	default:
		return DependencyBlocked
	}
}

func resolveNextRecommended(d Dependencies, applyState ApplyState) string {
	switch {
	case d.Proposal == DependencyReady:
		return string(PhasePropose)
	case d.Specs == DependencyReady:
		return string(PhaseSpec)
	case d.Design == DependencyReady:
		return string(PhaseDesign)
	case d.Tasks == DependencyReady:
		return string(PhaseTasks)
	case d.Apply == DependencyReady:
		return string(PhaseApply)
	case d.Verify == DependencyReady:
		return string(PhaseVerify)
	case d.Verify == DependencyAllDone && applyState == ApplyAllDone:
		return string(PhaseArchive)
	default:
		return "resolve-blockers"
	}
}

func artifactBlockedReasons(artifacts map[string]ArtifactState, tp TaskProgress) []string {
	var reasons []string
	for _, a := range []struct {
		key, file string
	}{
		{"proposal", "proposal.md"},
		{"specs", "spec.md"},
		{"design", "design.md"},
		{"tasks", "tasks.md"},
	} {
		if artifacts[a.key] != ArtifactDone {
			reasons = append(reasons, a.file+" is missing or partial.")
		}
	}
	if artifacts["tasks"] == ArtifactDone && tp.Total == 0 {
		reasons = append(reasons, "tasks.md has no markdown task checkboxes.")
	}
	return reasons
}

// singleArtifactState reports a single artifact's state from its resolved paths.
func singleArtifactState(paths []string) ArtifactState {
	if len(paths) == 0 {
		return ArtifactMissing
	}
	if hasContent(paths[0]) {
		return ArtifactDone
	}
	return ArtifactPartial
}

func hasContent(path string) bool {
	content, err := os.ReadFile(path)
	return err == nil && strings.TrimSpace(string(content)) != ""
}

func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func changeNamePtr(name string) *string {
	if name == "" {
		return nil
	}
	return &name
}

// baseStatus builds a status with the schema metadata, planning home, action
// context, and blocked defaults set; callers override the computed fields.
func baseStatus(cwd string, name, root *string, next string, reasons []string) Status {
	if reasons == nil {
		reasons = []string{}
	}
	return Status{
		SchemaName:    SchemaName,
		SchemaVersion: SchemaVersion,
		ChangeName:    name,
		ArtifactStore: artifactStore,
		PlanningHome:  PlanningHome{Mode: modeRepoLocal, Path: OpenSpecDir(cwd)},
		ChangeRoot:    root,
		ArtifactPaths: ArtifactPaths{}.withArrays(),
		Artifacts: map[string]ArtifactState{
			"proposal": ArtifactMissing, "specs": ArtifactMissing, "design": ArtifactMissing,
			"tasks": ArtifactMissing, "applyProgress": ArtifactMissing, "verifyReport": ArtifactMissing,
		},
		TaskProgress: TaskProgress{},
		Dependencies: Dependencies{
			Proposal: DependencyBlocked, Specs: DependencyBlocked, Design: DependencyBlocked,
			Tasks: DependencyBlocked, Apply: DependencyBlocked, Verify: DependencyBlocked, Archive: DependencyBlocked,
		},
		ApplyState: ApplyBlocked,
		ActionContext: ActionContext{
			Mode: modeRepoLocal, WorkspaceRoot: cwd, AllowedEditRoots: []string{cwd},
		},
		NextRecommended: next,
		BlockedReasons:  reasons,
	}
}

// blockedStatus is a base status with no resolved change root.
func blockedStatus(cwd string, name *string, next string, reasons []string) Status {
	return baseStatus(cwd, name, nil, next, reasons)
}

var taskCheckbox = regexp.MustCompile(`^\s*(?:[-*]|\d+[.)])\s+\[([ xX])\]`)

// countTaskProgress counts markdown task checkboxes in tasks.md.
func countTaskProgress(tasksPath string) TaskProgress {
	if tasksPath == "" {
		return TaskProgress{}
	}
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		return TaskProgress{}
	}
	var tp TaskProgress
	for _, line := range strings.Split(string(content), "\n") {
		m := taskCheckbox.FindStringSubmatch(line)
		if len(m) == 0 {
			continue
		}
		tp.Total++
		if m[1] == "x" || m[1] == "X" {
			tp.Completed++
		} else {
			tp.Pending++
		}
	}
	tp.AllComplete = tp.Total > 0 && tp.Pending == 0
	return tp
}

var (
	reportPassPattern     = regexp.MustCompile(`(?i)\b(?:PASS|PASSED|SUCCESS|SUCCESSFUL)\b`)
	reportFailPattern     = regexp.MustCompile(`(?i)\b(?:FAIL|FAILED|FAILING|FAILURE|BLOCKED|UNTESTED)\b`)
	reportPendingPattern  = regexp.MustCompile(`(?i)\b(?:TODO|PENDING)\b`)
	reportNegationPattern = regexp.MustCompile(`(?i)\bnot\s+(?:pass|passed|passing|successful|complete|completed)\b|\b(?:pass|passed|success|successful|complete|completed)\s*:\s*no\b`)
	reportFailedCount     = regexp.MustCompile(`(?i)\b(?:failed\s*:\s*|\b)(\d+)\s*failed\b|\bfailed\s*:\s*(\d+)\b`)
	reportCritical        = regexp.MustCompile(`(?i)\bCRITICAL\b`)
	reportBenign          = regexp.MustCompile(`(?i)^(?:none|no|n/a|0(?:\s+\w+)?)\.?$`)
)

// reportIsClearlyPassing reports whether verify-report.md has an explicit pass
// signal and no blocker signal. A missing or empty report is not passing.
func reportIsClearlyPassing(path string) bool {
	if path == "" {
		return false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(content)
	if strings.TrimSpace(text) == "" {
		return false
	}
	hasPass := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if reportLineHasBlocker(line) {
			return false
		}
		if reportPassPattern.MatchString(line) && !reportNegationPattern.MatchString(line) {
			hasPass = true
		}
	}
	return hasPass
}

func reportLineHasBlocker(line string) bool {
	if reportNegationPattern.MatchString(line) || reportPendingPattern.MatchString(line) {
		return true
	}
	// "N failed" / "failed: N" with N != 0 is a blocker.
	if m := reportFailedCount.FindStringSubmatch(line); m != nil {
		for _, g := range m[1:] {
			if g != "" && g != "0" {
				return true
			}
		}
	}
	// A CRITICAL field with a non-benign value blocks (e.g. "CRITICAL: null deref"),
	// but "CRITICAL: none" or "CRITICAL: 0" does not.
	if reportCritical.MatchString(line) {
		if _, value, ok := splitField(line); ok {
			if !reportBenign.MatchString(strings.TrimSpace(value)) {
				return true
			}
		} else {
			return true
		}
	}
	return reportFailPattern.MatchString(line)
}

var fieldPattern = regexp.MustCompile(`^\s*(?:[-*]\s+)?(?:\*\*)?([A-Za-z][A-Za-z\s-]*?)(?:\*\*)?\s*:\s*(.*)$`)

// splitField parses a "Label: value" markdown line.
func splitField(line string) (label, value string, ok bool) {
	m := fieldPattern.FindStringSubmatch(line)
	if len(m) != 3 {
		return "", "", false
	}
	return m[1], m[2], true
}
