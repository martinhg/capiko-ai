# Design: SDD status Engram fallback

## Context

`internal/sddstatus` is capiko's native SDD engine. It is purely file-based:
`Resolve` calls `selectChange`, which lists `openspec/changes/<change>/`, and every
artifact state is read from disk. The artifact store is the hardcoded
`const artifactStore = "openspec"` (status.go L72). The engine has no notion of
Engram.

This design adds a files-first, Engram-fallback resolution path. When the file path
finds no matching change but the change's Engram observations (`sdd/<change>/*`)
exist locally, reconstruct an equivalent `capiko.sdd-status` from those observations
instead of returning a blind "no active change". OpenSpec files stay canonical;
Engram is read only as a graceful-degradation fallback at the two `sdd-new` branches.

Scope is `internal/sddstatus` only, plus a bounded note in
`internal/catalog/skills/sdd-shared/sdd-status-contract.md`. No skill or agent
changes. No write path. This is a port of gentle-ai PR #957 adapted to capiko's real
`Resolve`/`selectChange` control flow.

### Grounded facts (verified against the real code, not gentle-ai's)

- `selectChange(cwd, requested) (name, blocked string, reasons []string, err error)`
  returns the routing token `"sdd-new"` in exactly two branches: requested change
  not in active list (status.go L185) and zero active changes (L189). Ambiguity
  returns `"select-change"` (L193) — and that branch means files DO exist on disk.
- `Resolve` branches on `blocked != ""` at L131-133 and returns
  `blockedStatus(cwd, changeNamePtr(changeName), blocked, reasons)`.
- `baseStatus` (L333) hardcodes `ArtifactStore: artifactStore` and
  `PlanningHome: {Mode: modeRepoLocal, Path: OpenSpecDir(cwd)}`. `Status.ArtifactStore`
  is a plain `string` (L95) — no enum.
- `countTaskProgress(tasksPath string)` (L371) does IO then loops `taskCheckbox`
  over the content. It splits cleanly into an IO-free text core + a thin path reader.
- `reportIsClearlyPassing(path string)` (L408) does IO then runs its line loop on the
  text. It splits cleanly into an IO-free text core + a thin path reader.
- `singleArtifactState(paths []string)` (L302) is path/IO based — the Engram path
  needs a content-based equivalent.
- `ResolveOptions` has only `Cwd` and `ChangeName`. There is **no** `IncludeInstructions`
  and `render.go` has no `renderPhaseInstructions`. Instruction rendering is fully
  separate (`RenderMarkdown`/`RenderDispatcherMarkdown`). So `resolveEngramStatus`
  drops gentle-ai's `includeInstructions` parameter — see ADR-4.
- There is **no** `emptyArtifactPaths` helper. Empty paths are produced by
  `ArtifactPaths{}.withArrays()` (paths.go L37).
- The package has **no** YAML reader. Config gating uses a narrow regex, not a YAML
  dependency.
- The `runOut` seam in `internal/engram/version.go` (`var runOut = func(...) (string, error)`)
  is the canonical exec test-seam pattern to mirror.

## Goals / Non-Goals

Goals:
- Recover an in-flight change from synced Engram memory when its OpenSpec files are
  absent, routing it exactly as the file path would.
- Zero behavior change when gating is off (byte-for-byte today's output).
- Hermetic tests: never shell out to the real `engram` binary.

Non-Goals:
- Engram is not a primary store; no write path; no `engram` binary management.
- No phase-skill/agent branching on artifact-store mode.
- `SchemaName`/`SchemaVersion` and JSON shape unchanged.

## Architecture

### Insertion point — inside `Resolve`, not `selectChange` (ADR-1)

The fallback fires inside `Resolve`, immediately before the existing
`return blockedStatus(...)`, and only when the routing token is `"sdd-new"`.
`selectChange` stays pure (it only returns routing tokens; it cannot carry a full
reconstructed `Status`). The `"select-change"` branch is intentionally excluded —
ambiguity means matching files exist on disk, so files win.

```go
	changeName, blocked, reasons, err := selectChange(cwd, options.ChangeName)
	if err != nil {
		return Status{}, err
	}
	if blocked != "" {
		// Files-first: only the two sdd-new branches (no active change / requested
		// change absent) fall back to Engram. The ambiguous select-change branch
		// means matching OpenSpec files exist on disk, so files win.
		if blocked == string(routeSDDNew) {
			if st, ok := resolveEngramStatus(cwd, options.ChangeName); ok {
				return st, nil
			}
		}
		return blockedStatus(cwd, changeNamePtr(changeName), blocked, reasons), nil
	}
```

(`routeSDDNew` is just the existing `"sdd-new"` string; a named const is optional —
a literal `"sdd-new"` comparison is acceptable and keeps the diff minimal.)

The fallback never returns an error to `Resolve`: any failure (gating off, binary
absent, non-zero exit, malformed JSON, no matching change) yields `ok == false` and
the normal blocked status is returned. `resolveEngramStatus` therefore returns
`(Status, bool)` — error handling is internal and always degrades to `ok == false`.

### `ArtifactStore` representation (minimal) (ADR-2 companion)

Add one const beside the existing one; do not introduce an enum and do not change
`baseStatus`:

```go
const (
	artifactStore        = "openspec" // unchanged default used by baseStatus
	ArtifactStoreEngram  = "engram"   // origin flag set only on the Engram path
)
```

The Engram path builds its `Status` through the **same** `baseStatus` (so it inherits
schema, action context, and array-safe defaults), then overrides three fields:

```go
	status := baseStatus(cwd, &change, &root, nextRecommended, blockedReasons)
	status.ArtifactStore   = ArtifactStoreEngram
	status.PlanningHome.Path = "engram:sdd"
	// ChangeRoot already &("engram:sdd/" + change) via baseStatus(root)
```

The JSON field stays a plain string — consumers that read `"openspec"` keep working;
they may now also observe `"engram"`.

### `resolveEngramStatus` flow

```go
// resolveEngramStatus reconstructs a status from Engram observations when the file
// path found no matching change. It never errors: any failure degrades to ok=false
// so Resolve returns the normal blocked status. Files stay canonical.
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
	// select-change blocked status (with the names) rather than a generic sdd-new.
	// ok=true so Resolve uses this status. ArtifactStore stays "openspec" (via
	// baseStatus) — SC-06 requires ArtifactStore != "engram" for this case.
	if len(changes) > 1 && requested == "" {
		reasons := []string{"Multiple Engram SDD changes found. Specify which to resume: " + strings.Join(changes, ", ") + "."}
		return blockedStatus(cwd, nil, "select-change", reasons), true
	}

	change, ok := selectEngramChange(changes, requested)
	if !ok {
		return Status{}, false // zero changes, or requested not found in Engram
	}

	artifacts := engramArtifactsForChange(obs, change, project)
	tasksText  := engramArtifactContent(obs, change, project, "tasks")
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
	status.ArtifactStore     = ArtifactStoreEngram
	status.PlanningHome.Path = "engram:sdd"
	status.ArtifactPaths     = engramArtifactPaths(change, artifacts).withArrays()
	status.Artifacts         = artifacts
	status.TaskProgress      = taskProgress
	status.Dependencies      = dependencies
	status.ApplyState        = applyState
	return status, true
}
```

Note this block is a structural twin of `Resolve` L135-168: the `coreReady`,
`applyState`, `blockedReasons`, verify-not-passing, `dependencies`, and
`nextRecommended` derivations are **identical** and call the **same** unchanged
helpers. Only the inputs (Engram content vs files) and the three origin overrides
differ. This is what guarantees `nextRecommended` parity with the file path.

### Helper functions

**`shouldTryEngram(cwd string) bool`** — gating. True when ANY of:

```go
func shouldTryEngram(cwd string) bool {
	if os.Getenv("CAPIKO_SDD_STATUS_ENGRAM") != "" {
		return true
	}
	if info, err := os.Stat(filepath.Join(cwd, ".engram")); err == nil && info.IsDir() {
		return true
	}
	return configArtifactStoreIsEngram(cwd)
}
```

**`configArtifactStoreIsEngram(cwd string) bool`** — reads `openspec/config.yaml` then
`openspec/config.yml`, matches `artifact_store:`/`artifactStore:` and checks the value
contains `engram` or `hybrid` (case-insensitive). No YAML dependency:

```go
var artifactStoreRe = regexp.MustCompile(`(?mi)^\s*artifact[_]?store\s*:\s*["']?([A-Za-z]+)`)
```

**`engramObservation`** — the export record shape:

```go
type engramObservation struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Project string `json:"project"`
	Scope   string `json:"scope"`
}
```

**Export seam** — mirrors `internal/engram` `runOut`, but at the observation boundary
so tests inject parsed records without temp files or JSON marshalling:

```go
// engramExport is a test seam. Tests swap it to return canned observations; the
// real implementation shells out to the engram binary. Tests NEVER shell out.
var engramExport = exportEngramObservations

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
```

**`inferEngramProject(cwd string) string`** — `ENGRAM_PROJECT` env → git remote
owner/repo from `.git/config` → lowercased dir basename:

```go
func inferEngramProject(cwd string) string {
	if p := strings.TrimSpace(os.Getenv("ENGRAM_PROJECT")); p != "" {
		return p
	}
	if p := projectFromGitConfig(cwd); p != "" {
		return p
	}
	return strings.ToLower(filepath.Base(cwd))
}
```

**`projectFromGitConfig(cwd string) string`** — read `.git/config`, find the
`[remote "origin"]` url, extract `owner/repo` (strip trailing `.git`), lowercased.
Returns `""` when absent/unparseable (fail-safe — falls through to dir basename).
A narrow regex over the remote url (`[:/]([^/:]+/[^/]+?)(?:\.git)?$`) is enough; no
git binary shell-out.

**`titleRe`** — parse observation titles into `(change, artifactType)`:

```go
var titleRe = regexp.MustCompile(`^sdd/([^/]+)/(proposal|spec|design|tasks|apply-progress|verify-report|state)$`)
```

The artifact type maps to the `artifacts` map key:
`proposal→proposal`, `spec→specs`, `design→design`, `tasks→tasks`,
`apply-progress→applyProgress`, `verify-report→verifyReport`. `state` is used for
change discovery but is not an artifact in the map.

**`engramObservationMatchesProject(obs engramObservation, project string) bool`** —
case-insensitive project equality AND `scope != "personal"`:

```go
func engramObservationMatchesProject(obs engramObservation, project string) bool {
	return strings.EqualFold(obs.Project, project) && !strings.EqualFold(obs.Scope, "personal")
}
```

**`collectEngramChanges(obs []engramObservation, project string) []string`** — distinct,
sorted change names from titles of project-matching, non-personal observations.

**`selectEngramChange(changes []string, requested string) (string, bool)`** —
files-first, fail-safe selection:
- `requested != ""`: return it iff present in `changes`, else `("", false)`.
- `requested == ""`: exactly one change → return it; zero or multiple → `("", false)`.
  `selectEngramChange` itself never fabricates a selection. The multiple-changes-no-request
  case is intercepted upstream in `resolveEngramStatus` (the ambiguity branch above), which
  short-circuits to a `select-change` blocked status listing the names — so this helper is
  reached only for the unambiguous (single or explicitly-requested) cases.

**`engramArtifactContent(obs, change, project, artifactType string) string`** — content
of the single matching observation (by `titleRe` + project match), or `""` if absent.

**`engramArtifactState(content string, present bool) ArtifactState`** — content-based
twin of `singleArtifactState`:

```go
func engramArtifactState(content string, present bool) ArtifactState {
	if !present {
		return ArtifactMissing
	}
	if strings.TrimSpace(content) != "" {
		return ArtifactDone
	}
	return ArtifactPartial
}
```

**`engramArtifactsForChange(obs, change, project string) map[string]ArtifactState`** —
build the same six-key map the file path produces, via `engramArtifactState` over the
matching observations for each artifact key.

**`engramArtifactPaths(change string, artifacts map[string]ArtifactState) ArtifactPaths`** —
sentinel `engram:sdd/<change>/<artifact>` paths for present (non-missing) artifacts,
empty slices otherwise; finalized with `.withArrays()`. Sentinel filenames mirror the
on-disk names (`proposal`, `spec`, `design`, `tasks`, `apply-progress`,
`verify-report`) so the prefix is the only difference. The `engram:` prefix signals
consumers these are not filesystem paths (documented in the contract note).

**`countTaskProgressText(content string) TaskProgress`** — see refactor below.

### Share-logic refactor (file ↔ text) (ADR-3)

Two existing path-based functions are split so the Engram path reuses the exact same
parsing/scoring logic. **File-path behavior is preserved byte-for-byte** — the path
wrappers keep their early-return semantics (empty path → empty; read error → empty/
not-passing) and delegate the pure logic to a text core.

```go
func countTaskProgress(tasksPath string) TaskProgress {
	if tasksPath == "" {
		return TaskProgress{}
	}
	content, err := os.ReadFile(tasksPath)
	if err != nil {
		return TaskProgress{}
	}
	return countTaskProgressText(string(content))
}

// countTaskProgressText counts markdown task checkboxes in arbitrary text (a tasks.md
// file body or an Engram tasks observation). Shares the taskCheckbox regex.
func countTaskProgressText(content string) TaskProgress {
	var tp TaskProgress
	for _, line := range strings.Split(content, "\n") {
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
```

```go
func reportIsClearlyPassing(path string) bool {
	if path == "" {
		return false
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return reportTextIsClearlyPassing(string(content))
}

// reportTextIsClearlyPassing holds the line-scan logic previously inline in
// reportIsClearlyPassing; the path wrapper just supplies the text.
func reportTextIsClearlyPassing(text string) bool {
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
```

`reportLineHasBlocker`, `splitField`, all `report*Pattern` regexes, `taskCheckbox`,
`resolveApplyState`, `resolveDependencies`, `resolveNextRecommended`,
`artifactBlockedReasons`, and `baseStatus` are reused **unchanged**.

## Data flow

```
Resolve(cwd, change)
  └─ selectChange → blocked == "sdd-new"?
        └─ resolveEngramStatus(cwd, requested)
              ├─ shouldTryEngram(cwd)            (env | .engram | config)        → false → ok=false
              ├─ engramExport()  [SEAM]          → []engramObservation            → err  → ok=false
              ├─ inferEngramProject(cwd)         (ENGRAM_PROJECT | git | basename)
              ├─ collectEngramChanges            → []change
              ├─ len>1 && no request?            → select-change blocked status, ok=true
              ├─ selectEngramChange(requested)                                    → none → ok=false
              ├─ engramArtifactsForChange        → map[string]ArtifactState
              ├─ countTaskProgressText(tasks)    (shared core)
              ├─ reportTextIsClearlyPassing(vr)  (shared core)
              ├─ resolveApplyState / resolveDependencies / resolveNextRecommended (REUSED)
              └─ baseStatus + 3 origin overrides → Status, ok=true
        ok? → return engram Status : return blockedStatus(...)  (unchanged)
```

## Engram export contract

`engram export <tmpfile>` writes `{"observations":[{"title","content","project","scope"}]}`
to the given path. `exportEngramObservations` creates a temp file, runs the command,
reads and unmarshals it. Any failure mode — binary not on PATH, non-zero exit,
unreadable/malformed JSON — returns a non-nil error, which `resolveEngramStatus`
converts to `ok == false`. The fallback can therefore only ever *upgrade* a blind
`sdd-new` into a routed status; it can never crash or change the file-path result.

Schema drift risk: if the export schema changes, unmarshal yields no usable
observations and the fallback degrades to today's blocked status (see proposal Risks).

## Test strategy (Strict TDD, red-first)

All tests live in `internal/sddstatus/status_test.go`, drive the `engramExport` seam,
set gating via env/`.engram`/config in a `t.TempDir()` workspace, and restore the seam
with `t.Cleanup`. No test invokes the real `engram` binary. `internal/tui` is
untouched, so **no golden files change** (confirm `go test ./internal/tui` unchanged
at apply).

Test helper (mirrors the existing `change` helper):

```go
func withEngram(t *testing.T, obs []engramObservation) {
	t.Helper()
	prev := engramExport
	engramExport = func() ([]engramObservation, error) { return obs, nil }
	t.Cleanup(func() { engramExport = prev })
}
```

Coverage mapped to the 10 spec requirements (write the failing test first for each):

1. **Gating off → byte-for-byte today.** No env, no `.engram`, no config; Engram has
   the change. Assert result equals the existing `sdd-new` blocked status
   (`engramExport` must not even be consulted — assert via a seam that fails the test
   if called).
2. **Gating via env** `CAPIKO_SDD_STATUS_ENGRAM` → fallback attempted and resolves.
3. **Gating via `.engram` dir** → fallback attempted and resolves.
4. **Gating via config** `artifact_store: hybrid`/`engram` in `openspec/config.yaml`
   → fallback attempted; `openspec` value does NOT gate on.
5. **No-change branch reconstruction** — zero active files, Engram has one change with
   full planning artifacts + one unchecked task → `artifactStore == "engram"`,
   `changeRoot == "engram:sdd/<change>"`, `planningHome.path == "engram:sdd"`,
   `nextRecommended == apply`.
6. **Requested-change-not-found branch** — `ChangeName: "x"` absent on disk, present in
   Engram → resolves to that change; absent in both → unchanged `sdd-new`.
7. **`nextRecommended` parity** — table mirroring the file-path routing tests
   (propose/spec/design/tasks/apply/verify/archive) driven from Engram content; assert
   identical `nextRecommended` and dependency states as the equivalent file test.
8. **Task progress text parsing** — `countTaskProgressText` parity with `tasks.md`
   parsing (mixed `[ ]`/`[x]`, no-checkbox prose → `ApplyBlocked` + `resolve-blockers`).
9. **Project inference branches** — `ENGRAM_PROJECT` match; git-config owner/repo match;
   dir-basename match; and a mismatch → fail-safe `ok=false` (normal `sdd-new`).
10. **Degradation** — `engramExport` returns error / empty / personal-scope-only →
    `ok=false`, normal blocked status; verify-report present-but-not-passing path yields
    the not-clearly-passing blocker.
11. **Ambiguity** — multiple Engram changes with no request → `select-change` blocked
    status (`ok=true`) listing the names, `ArtifactStore` stays `"openspec"` (SC-06).

## Decisions (ADRs)

### ADR-1: Fallback fires inside `Resolve` at the `sdd-new` branch, not in `selectChange`
- **Decision**: Call `resolveEngramStatus` inside `Resolve`, right before the existing
  `return blockedStatus(...)`, gated on `blocked == "sdd-new"`.
- **Rationale**: `selectChange` returns only routing tokens
  `(name, blocked, reasons, err)` and cannot carry a full reconstructed `Status`
  without a signature change that would entangle selection with reconstruction. The
  `Resolve` seam is one `if` around the existing return — minimal, and keeps
  `selectChange` pure and its tests intact.
- **Files-first**: only the two `sdd-new` branches fall back. The `select-change`
  (ambiguous) branch is excluded because ambiguity means matching files exist on disk.
- **Rejected**: (a) fallback inside `selectChange` — needs a richer return type and
  couples selection with IO/exec; (b) a parallel top-level resolver branch — duplicates
  the `baseStatus` assembly and the derivation block.

### ADR-2: Origin via a new const + post-`baseStatus` override, no enum
- **Decision**: Add `ArtifactStoreEngram = "engram"` and override
  `status.ArtifactStore`, `status.PlanningHome.Path`, and the `root` (`ChangeRoot`)
  after building through the unchanged `baseStatus`.
- **Rationale**: `Status.ArtifactStore` is a plain `string`; an enum is unnecessary and
  would churn the JSON contract. Reusing `baseStatus` inherits schema/action-context/
  array-safe defaults for free, so the Engram and file paths cannot diverge on those.
- **Rejected**: introducing an `ArtifactStore` enum type (breaks the plain-string field,
  larger blast radius); threading a store flag through `baseStatus` (changes a shared
  signature used by every blocked status).

### ADR-3: Share file/text logic via extracted text cores
- **Decision**: Split `countTaskProgress`/`reportIsClearlyPassing` into IO-free text
  cores (`countTaskProgressText`, `reportTextIsClearlyPassing`) with thin path
  wrappers; reuse the shared regexes.
- **Rationale**: Parity between file and Engram routing must be structural, not
  re-implemented. A single source of truth for checkbox counting and pass detection
  prevents drift (a real risk called out in the proposal). The path wrappers keep their
  exact early-return semantics, so file-path behavior is unchanged.
- **Rejected**: duplicating the parsers for the text path (drift risk); changing the
  path-function signatures to accept text (breaks existing callers/tests in `Resolve`).

### ADR-4: Drop gentle-ai's `includeInstructions` parameter
- **Decision**: capiko's `resolveEngramStatus(cwd, requested string) (Status, bool)` —
  no `includeInstructions`.
- **Rationale**: capiko's `ResolveOptions` has no instructions flag and instruction
  rendering lives entirely in `render.go` (`RenderMarkdown`/`RenderDispatcherMarkdown`),
  decoupled from `Resolve`. Carrying the gentle-ai parameter would be dead weight.
  Returning `(Status, bool)` (no error) reflects the contract that the fallback never
  fails the caller — every internal error degrades to `ok == false`.

## Contract doc note

Add to `internal/catalog/skills/sdd-shared/sdd-status-contract.md`, under the
**Artifact Store** section, a bounded note: the store is file-first; the engine MAY
report `artifactStore: engram` with a `changeRoot` of `engram:sdd/<change>` and
`planningHome.path` of `engram:sdd` **only when** no matching OpenSpec change exists on
disk but synced Engram observations do. Parsers MUST treat an `engram:`-prefixed
`changeRoot` as a non-filesystem origin marker, not a path. This preserves the
canonical-files invariant: Engram is a read-only fallback, never a write target and
never preferred over files.

## Risks

- **Project-inference mismatch** is fail-safe but can miss a recoverable change saved
  under a different project key (covered by test #9).
- **Export schema drift** degrades to today's behavior, never a crash; confirm the
  `{"observations":[...]}` shape at apply.
- **Tasks-body formatting** differences between an Engram observation and `tasks.md`
  could skew progress; mitigated by reusing the exact `taskCheckbox` regex and test #8.
- **Contract tension**: the doc currently says "ALWAYS file-based"; the note must frame
  Engram strictly as a read-only fallback to avoid implying the invariant is weakened.
```
