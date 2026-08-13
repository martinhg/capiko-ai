package catalog

import (
	"reflect"
	"strings"
	"testing"

	"github.com/martinhg/capiko-ai/internal/agent"
	"github.com/martinhg/capiko-ai/internal/sddstatus"
	"github.com/martinhg/capiko-ai/internal/skill"
)

// jdJudgeNames is the canonical list of the two blind Judgment-Day judges.
var jdJudgeNames = []string{"capiko-jd-judge-a", "capiko-jd-judge-b"}

// TestLoadAgents_ReturnsTwelve exercises the embedded agents catalog at test
// time so any authoring error is caught before the binary ships.
func TestLoadAgents_ReturnsTwelve(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	if len(agents) != 12 {
		t.Fatalf("expected 12 agents from embedded catalog, got %d", len(agents))
	}
	for _, a := range agents {
		if a.Name == "" {
			t.Error("agent has empty name")
		}
		if a.Description == "" {
			t.Errorf("agent %q has empty description", a.Name)
		}
		if a.Content == "" {
			t.Errorf("agent %q has empty content", a.Name)
		}
	}
}

// TestJDJudges_NoEditTool pins the Judgment-Day judge agent shape: judges are
// blind adversarial reviewers that must never modify code, so their `tools:`
// list must contain read/search/execute but never edit.
func TestJDJudges_NoEditTool(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	byName := map[string]agent.Agent{}
	for _, a := range agents {
		byName[a.Name] = a
	}

	for _, name := range jdJudgeNames {
		a, ok := byName[name]
		if !ok {
			t.Errorf("judge %q not found in catalog", name)
			continue
		}
		fm, err := agent.ParseFrontmatter(a.Content)
		if err != nil {
			t.Errorf("judge %q: frontmatter parse error: %v", name, err)
			continue
		}
		if fm.UserInvocable {
			t.Errorf("judge %q: user-invocable must be false", name)
		}
		toolSet := map[string]bool{}
		for _, tool := range fm.Tools {
			toolSet[tool] = true
		}
		for _, want := range []string{"read", "search", "execute"} {
			if !toolSet[want] {
				t.Errorf("judge %q: tools must include %q, got %v", name, want, fm.Tools)
			}
		}
		if toolSet["edit"] {
			t.Errorf("judge %q: tools must NOT include edit, got %v", name, fm.Tools)
		}
	}
}

// TestJDFixAgent_ScopedToConfirmedIssues pins the fix agent's write access and
// its scope: it MUST declare edit (it needs write access to fix code) and its
// body MUST restrict fixes to issues both judges jointly confirmed, never
// open-ended changes.
func TestJDFixAgent_ScopedToConfirmedIssues(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var fix *agent.Agent
	for i := range agents {
		if agents[i].Name == "capiko-jd-fix-agent" {
			fix = &agents[i]
			break
		}
	}
	if fix == nil {
		t.Fatal("capiko-jd-fix-agent not found in catalog")
	}

	fm, err := agent.ParseFrontmatter(fix.Content)
	if err != nil {
		t.Fatalf("capiko-jd-fix-agent: frontmatter parse error: %v", err)
	}
	if fm.UserInvocable {
		t.Error("capiko-jd-fix-agent: user-invocable must be false")
	}
	toolSet := map[string]bool{}
	for _, tool := range fm.Tools {
		toolSet[tool] = true
	}
	for _, want := range []string{"read", "edit", "search", "execute"} {
		if !toolSet[want] {
			t.Errorf("capiko-jd-fix-agent: tools must include %q, got %v", want, fm.Tools)
		}
	}
	if !strings.Contains(fix.Content, "confirmed") {
		t.Error("capiko-jd-fix-agent body must restrict edits to jointly-confirmed issues")
	}
}

// TestCoordinator_JudgmentDayProtocol pins the coordinator's ownership of JD
// routing: the body must describe the protocol, launch both judges before a
// PR, and delegate only jointly-confirmed findings to the fix agent.
func TestCoordinator_JudgmentDayProtocol(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var coord string
	for _, a := range agents {
		if a.Name == "capiko-sdd-coordinator" {
			coord = a.Content
			break
		}
	}
	if coord == "" {
		t.Fatal("capiko-sdd-coordinator not found in catalog")
	}
	for _, want := range []string{
		"Judgment-Day Protocol",
		"capiko-jd-judge-a",
		"capiko-jd-judge-b",
		"capiko-jd-fix-agent",
		"confirmed",
	} {
		if !strings.Contains(coord, want) {
			t.Errorf("coordinator body must contain %q", want)
		}
	}
}

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
	if !byName["codebase-docs"] {
		t.Errorf("expected codebase-docs in catalog, got %v", byName)
	}
	if !byName["skill-creator"] {
		t.Errorf("expected skill-creator in catalog, got %v", byName)
	}
	if !byName["skill-registry"] {
		t.Errorf("expected skill-registry in catalog, got %v", byName)
	}

	// The SDD skills bundle must be present and parse.
	for _, name := range []string{
		"explore", "propose", "spec", "design", "tasks", "apply", "verify", "archive",
		"init", "onboard",
	} {
		if !byName["sdd-"+name] {
			t.Errorf("expected sdd-%s in catalog", name)
		}
	}

	// The 4R review skills must be present.
	for _, name := range []string{
		"review-risk", "review-readability", "review-reliability", "review-resilience",
	} {
		if !byName[name] {
			t.Errorf("expected %s in catalog", name)
		}
	}
}

// TestFourRReviewSkillsStructuredOutput pins the 4R review skills to their
// structured output contract: each must define a severity-tagged output format
// and a "no issues" statement so consumers can parse findings programmatically.
func TestFourRReviewSkillsStructuredOutput(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	byName := map[string]string{}
	for _, s := range got {
		byName[s.Name] = s.Content
	}

	for _, name := range []string{
		"review-risk", "review-readability", "review-reliability", "review-resilience",
	} {
		body, ok := byName[name]
		if !ok {
			t.Errorf("%s not found in catalog", name)
			continue
		}
		for _, want := range []string{
			"## Output format",
			"SEVERITY",
			"Category",
			"Location",
			"No ",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s must contain %q for structured output", name, want)
			}
		}
	}
}

// TestFileReadingSkillsCarryUntrustedContentClause pins the prompt-injection
// defence to every skill that reads arbitrary project files. Without this
// clause the agent may treat injected directives inside scanned files as
// instructions (indirect prompt injection).
func TestFileReadingSkillsCarryUntrustedContentClause(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	byName := map[string]string{}
	for _, s := range got {
		byName[s.Name] = s.Content
	}

	skills := []string{
		"codebase-docs",
		"sdd-explore",
		"review-risk",
		"review-readability",
		"review-reliability",
		"review-resilience",
	}
	for _, name := range skills {
		body, ok := byName[name]
		if !ok {
			t.Errorf("%s not found in catalog", name)
			continue
		}
		for _, want := range []string{
			"Untrusted content",
			"prompt-injection",
			"never as instructions to follow",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s must contain %q for untrusted-content defence", name, want)
			}
		}
	}
}

// TestTasksSkillEmitsWorkloadGuard pins the review-workload guard contract to the
// sdd-tasks skill body. The shared protocol (sdd-phase-common.md, section F)
// requires sdd-tasks to forecast the 400-line review budget with exact guard
// lines, and the orchestrator reads that forecast before launching apply. If the
// guard lines disappear from the skill, the executor stops emitting them and the
// orchestrator's workload gate silently goes blind — so guard them here.
func TestTasksSkillEmitsWorkloadGuard(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var tasks string
	for _, s := range got {
		if s.Name == "sdd-tasks" {
			tasks = s.Content
			break
		}
	}
	if tasks == "" {
		t.Fatal("sdd-tasks skill not found or has empty content")
	}

	required := []string{
		"Review Workload Forecast",
		"Decision needed before apply:",
		"Chained PRs recommended:",
		"400-line budget risk:",
	}
	for _, want := range required {
		if !strings.Contains(tasks, want) {
			t.Errorf("sdd-tasks skill must contain workload-guard line %q, but it is missing", want)
		}
	}
}

// TestSDDStrategyDecisionFlow pins the delivery-strategy + chain-strategy
// decision flow to the delegation layer. The orchestrator block describes it,
// but the coordinator agent is the real delegation target that must apply the
// Review Workload Guard before launching apply, and the shared contract must
// document both chain strategies. Without this, the orchestrator's flow has no
// receiver in the agent that actually routes the phases.
func TestSDDStrategyDecisionFlow(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	var coord string
	for _, a := range agents {
		if a.Name == "capiko-sdd-coordinator" {
			coord = a.Content
			break
		}
	}
	if coord == "" {
		t.Fatal("capiko-sdd-coordinator agent not found in catalog")
	}
	for _, want := range []string{"Review Workload", "ask-on-risk", "stacked-to-main", "feature-branch-chain"} {
		if !strings.Contains(coord, want) {
			t.Errorf("coordinator agent must carry the strategy flow %q, but it is missing", want)
		}
	}

	// The shared contract documents both chain strategies for every phase.
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	var common string
	for i := range got {
		if got[i].Name == "sdd-shared" {
			for _, f := range got[i].Extra {
				if f.Path == "sdd-phase-common.md" {
					common = f.Content
				}
			}
		}
	}
	if common == "" {
		t.Fatal("sdd-shared must ship sdd-phase-common.md as an Extra file")
	}
	for _, want := range []string{"stacked-to-main", "feature-branch-chain"} {
		if !strings.Contains(common, want) {
			t.Errorf("sdd-phase-common.md must document chain strategy %q", want)
		}
	}
}

// TestApplySkillShipsStrictTDDReference pins the strict-TDD reference file to the
// sdd-apply bundle. The orchestrator's strict-TDD forwarding tells the apply
// executor to "follow strict-tdd.md"; if that reference file stops shipping with
// the skill, the forwarding dangles and the executor falls back to ad-hoc TDD.
// Guard both that the file ships as an Extra and that the SKILL.md points to it.
func TestApplySkillShipsStrictTDDReference(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var apply *skill.Skill
	for i := range got {
		if got[i].Name == "sdd-apply" {
			apply = &got[i]
			break
		}
	}
	if apply == nil {
		t.Fatal("sdd-apply skill not found in catalog")
	}

	var ref string
	for _, f := range apply.Extra {
		if f.Path == "strict-tdd.md" {
			ref = f.Content
			break
		}
	}
	if ref == "" {
		t.Fatal("sdd-apply must ship strict-tdd.md as an Extra file with content")
	}

	// The reference must carry the core RED→GREEN protocol vocabulary.
	for _, want := range []string{"RED", "GREEN", "failing test"} {
		if !strings.Contains(ref, want) {
			t.Errorf("strict-tdd.md must cover %q", want)
		}
	}

	// The SKILL.md must point the executor at the reference file.
	if !strings.Contains(apply.Content, "strict-tdd.md") {
		t.Error("sdd-apply SKILL.md must reference strict-tdd.md so the executor loads it")
	}
}

// TestVerifySkillShipsStrictTDDReference pins the strict-TDD verification
// reference to the sdd-verify bundle. The orchestrator's strict-TDD forwarding
// tells the verify executor to follow strict-tdd-verify.md; if it stops shipping,
// the forwarding dangles and TDD compliance goes unchecked at verify time.
func TestVerifySkillShipsStrictTDDReference(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	var verify *skill.Skill
	for i := range got {
		if got[i].Name == "sdd-verify" {
			verify = &got[i]
			break
		}
	}
	if verify == nil {
		t.Fatal("sdd-verify skill not found in catalog")
	}

	var ref string
	for _, f := range verify.Extra {
		if f.Path == "strict-tdd-verify.md" {
			ref = f.Content
			break
		}
	}
	if ref == "" {
		t.Fatal("sdd-verify must ship strict-tdd-verify.md as an Extra file with content")
	}

	// The reference must carry the core verification vocabulary: it audits whether
	// the change was built test-first and grades findings by severity.
	for _, want := range []string{"test-first", "CRITICAL", "WARNING"} {
		if !strings.Contains(ref, want) {
			t.Errorf("strict-tdd-verify.md must cover %q", want)
		}
	}

	// The SKILL.md must point the executor at the reference file.
	if !strings.Contains(verify.Content, "strict-tdd-verify.md") {
		t.Error("sdd-verify SKILL.md must reference strict-tdd-verify.md so the executor loads it")
	}
}

// ---------------------------------------------------------------------------
// Content-quality regression tests: field-reported SDD pipeline bugs
// ---------------------------------------------------------------------------
// These tests pin the instructional safeguards added after three bugs surfaced
// in real Copilot CLI usage. Without them, a future edit to the shared protocol
// or a specific skill could silently re-introduce the failure mode.

// TestPhaseCommonArtifactVerification pins the "verify after write" and "create
// directory" instructions in § D. Without these, an executor agent can claim
// success without the artifact actually existing on disk (phantom-file bug).
func TestPhaseCommonArtifactVerification(t *testing.T) {
	common := loadPhaseCommon(t)
	for _, want := range []string{
		"ensure the target directory exists",
		"reading it back",
		"status: blocked",
	} {
		if !strings.Contains(common, want) {
			t.Errorf("sdd-phase-common.md § D must contain %q (phantom-file prevention)", want)
		}
	}
}

// TestPhaseCommonExampleDataGuard pins § H (Example Data) which prevents
// design/verify phases from blocking on format validation of test data
// (UUID false-positive bug).
func TestPhaseCommonExampleDataGuard(t *testing.T) {
	common := loadPhaseCommon(t)
	for _, want := range []string{
		"Example Data",
		"format validation",
		"UUID",
	} {
		if !strings.Contains(common, want) {
			t.Errorf("sdd-phase-common.md must contain %q (example-data guard)", want)
		}
	}
}

// TestArchiveSkillCreatesDirectory pins the explicit "create archive directory"
// instruction in sdd-archive step 4. Without it, the move fails when
// openspec/changes/archive/ does not exist (archive-mkdir bug).
func TestArchiveSkillCreatesDirectory(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	var archive string
	for _, s := range got {
		if s.Name == "sdd-archive" {
			archive = s.Content
			break
		}
	}
	if archive == "" {
		t.Fatal("sdd-archive skill not found in catalog")
	}
	if !strings.Contains(archive, "Create") || !strings.Contains(archive, "archive/") {
		t.Error("sdd-archive must instruct creating the archive directory before moving")
	}
}

// TestPhaseCommonSectionsAtoI verifies that sdd-phase-common.md contains all
// sections A through I. If a section is removed or renamed, every SDD skill's
// Gate reference (§ A–I) becomes a dangling pointer.
func TestPhaseCommonSectionsAtoI(t *testing.T) {
	common := loadPhaseCommon(t)
	sections := []string{
		"## A.", "## B.", "## C.", "## D.", "## E.",
		"## F.", "## G.", "## H.", "## I.",
	}
	for _, s := range sections {
		if !strings.Contains(common, s) {
			t.Errorf("sdd-phase-common.md must contain section %q", s)
		}
	}
}

// TestSDDSkillsReferenceCorrectSectionRange verifies that every SDD phase
// skill's Gate references the actual section range in sdd-phase-common.md.
// If a new section is added to the common file but the Gate references aren't
// updated, executors skip the new section.
func TestSDDSkillsReferenceCorrectSectionRange(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	sddPhases := []string{
		"sdd-explore", "sdd-propose", "sdd-spec", "sdd-design",
		"sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive",
	}
	for _, name := range sddPhases {
		for _, s := range got {
			if s.Name == name {
				if !strings.Contains(s.Content, "§ A–I") {
					t.Errorf("%s Gate must reference § A–I (current section range)", name)
				}
				break
			}
		}
	}
}

// TestProposeSkillHasStepZero pins the interactive proposal question round
// (Step 0) that was added as part of K-18. Without it, the propose phase
// writes proposals without gathering business context first.
func TestProposeSkillHasStepZero(t *testing.T) {
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	var propose string
	for _, s := range got {
		if s.Name == "sdd-propose" {
			propose = s.Content
			break
		}
	}
	if propose == "" {
		t.Fatal("sdd-propose skill not found in catalog")
	}
	for _, want := range []string{
		"Step 0",
		"Proposal Questions",
		"Non-interactive fallback",
	} {
		if !strings.Contains(propose, want) {
			t.Errorf("sdd-propose must contain %q (Step 0 question round)", want)
		}
	}
}

// loadPhaseCommon is a test helper that loads sdd-phase-common.md from the
// sdd-shared skill's Extra files.
func loadPhaseCommon(t *testing.T) string {
	t.Helper()
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	for _, s := range got {
		if s.Name == "sdd-shared" {
			for _, f := range s.Extra {
				if f.Path == "sdd-phase-common.md" {
					return f.Content
				}
			}
		}
	}
	t.Fatal("sdd-shared must ship sdd-phase-common.md as an Extra file")
	return ""
}

// loadStatusContract is a test helper that loads sdd-status-contract.md from
// the sdd-shared skill's Extra files.
func loadStatusContract(t *testing.T) string {
	t.Helper()
	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	for _, s := range got {
		if s.Name == "sdd-shared" {
			for _, f := range s.Extra {
				if f.Path == "sdd-status-contract.md" {
					return f.Content
				}
			}
		}
	}
	t.Fatal("sdd-shared must ship sdd-status-contract.md as an Extra file")
	return ""
}

// collectJSONFieldNames recursively extracts JSON tag names from a struct
// type, descending into nested (and pointer-to) struct fields. Map- and
// slice-typed fields are recorded by their own JSON name but not descended
// into, since their element names are not compile-time struct metadata.
func collectJSONFieldNames(t reflect.Type) []string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var names []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name := field.Name
		if tag != "" {
			if parts := strings.Split(tag, ","); parts[0] != "" {
				name = parts[0]
			}
		}
		names = append(names, name)

		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			names = append(names, collectJSONFieldNames(ft)...)
		}
	}
	return names
}

// TestStatusContractCoversAllStructFields is a parity guard between the
// sddstatus.Status struct (the actual capiko-ai sdd-status --json output) and
// the sdd-status-contract.md fallback reference consumed when the binary is
// not on PATH. It walks Status and its nested structs (PlanningHome,
// ActionContext, TaskProgress, Dependencies, ArtifactPaths) via reflection and
// asserts every JSON field name appears in the contract text, plus the
// ArtifactState/DependencyState/ApplyState enum values. The Artifacts map's
// six keys are not visible to reflection (map keys are a runtime detail, not
// struct metadata), but they are identical to ArtifactPaths's JSON tags,
// which are already covered by this walk. If a field is ever added to the Go
// struct without updating the contract, this test fails — forcing the
// contract to be updated, not this test relaxed.
func TestStatusContractCoversAllStructFields(t *testing.T) {
	contract := loadStatusContract(t)

	fields := collectJSONFieldNames(reflect.TypeOf(sddstatus.Status{}))
	if len(fields) == 0 {
		t.Fatal("collectJSONFieldNames returned no fields for sddstatus.Status; reflection walk is broken")
	}
	for _, f := range fields {
		if !strings.Contains(contract, f) {
			t.Errorf("sdd-status-contract.md is missing struct field %q", f)
		}
	}

	enumValues := []string{
		string(sddstatus.ArtifactMissing),
		string(sddstatus.ArtifactPartial),
		string(sddstatus.ArtifactDone),
		string(sddstatus.DependencyBlocked),
		string(sddstatus.DependencyReady),
		string(sddstatus.DependencyAllDone),
		string(sddstatus.ApplyBlocked),
		string(sddstatus.ApplyReady),
		string(sddstatus.ApplyAllDone),
	}
	seen := map[string]bool{}
	for _, v := range enumValues {
		if seen[v] {
			continue
		}
		seen[v] = true
		if !strings.Contains(contract, v) {
			t.Errorf("sdd-status-contract.md is missing enum value %q", v)
		}
	}
}

// TestSDDAgentsForwardStrictTDD pins the structural strict-TDD forwarding to the
// delegation targets. The orchestrator forwards `strict_tdd: true` into the
// apply/verify handoff; the worker .agent.md bodies are the direct delegation
// targets, so they MUST detect that signal and load the right reference file. The
// coordinator MUST carry the rule to forward it. Without this, the forwarding the
// orchestrator block promises has no receiver and the worker silently skips TDD.
func TestSDDAgentsForwardStrictTDD(t *testing.T) {
	agents, err := LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents error: %v", err)
	}
	byName := map[string]string{}
	for _, a := range agents {
		byName[a.Name] = a.Content
	}

	cases := []struct {
		name     string
		contains []string
	}{
		// Apply/verify workers detect the forwarded signal and load their protocol.
		{"capiko-sdd-apply", []string{"strict_tdd: true", "strict-tdd.md"}},
		{"capiko-sdd-verify", []string{"strict_tdd: true", "strict-tdd-verify.md"}},
		// The coordinator must carry the rule to forward the signal downstream.
		{"capiko-sdd-coordinator", []string{"strict_tdd: true", "forward"}},
	}
	for _, c := range cases {
		body, ok := byName[c.name]
		if !ok {
			t.Errorf("agent %q not found in catalog", c.name)
			continue
		}
		for _, want := range c.contains {
			if !strings.Contains(body, want) {
				t.Errorf("agent %q must carry strict-TDD forwarding %q, but it is missing", c.name, want)
			}
		}
	}
}
