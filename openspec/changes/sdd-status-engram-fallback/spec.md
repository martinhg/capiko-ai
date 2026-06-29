# Spec: SDD status Engram fallback

**Change:** `sdd-status-engram-fallback`
**Schema:** `capiko.sdd-status`
**Affected package:** `internal/sddstatus`

---

## What this change delivers

`Resolve` gains a files-first, Engram-fallback resolution path. When `selectChange` would
return a blocked status because the requested change is absent from OpenSpec, and gating
conditions indicate Engram is in use, the engine reconstructs the same `capiko.sdd-status`
from Engram observations (`sdd/<change>/*`) instead of returning the blind blocked result.
OpenSpec files remain canonical; Engram is consulted only when the files are not there.

---

## Non-negotiable invariants

These behaviors MUST NOT change under this spec:

| Invariant | Rule |
|---|---|
| `SchemaName` | Always `"capiko.sdd-status"` |
| `SchemaVersion` | Always `1` |
| JSON shape | Unchanged — no new top-level fields |
| Files stay canonical | When a matching OpenSpec change exists, the Engram path is never taken |
| No crash path | Any failure in the Engram path (absent binary, malformed JSON, schema drift) returns the same blocked status the file path would have returned — never a Go error to the caller |
| No write path | The fallback only reads; it never writes Engram or OpenSpec artifacts |
| TUI golden files | `internal/tui/testdata/*.golden` are expected to be untouched |

---

## REQ-1: Gating — off by default

When none of the gating conditions are satisfied, `Resolve` behavior is byte-for-byte
identical to the current implementation. The Engram export seam is never invoked.

**Gating OFF conditions (all must be absent):**

- The env var `CAPIKO_SDD_STATUS_ENGRAM` is not set
- No `.engram` directory exists at the workspace root
- `openspec/config.yaml` / `openspec/config.yml` does not exist OR does not contain
  `artifact_store:` / `artifactStore:` with value `engram` or `hybrid`

---

## REQ-2: Gating — on via any single trigger

Gating is ON when ANY ONE of the following conditions is true. Each trigger is
independently sufficient.

| Trigger | Condition |
|---|---|
| Env override | `CAPIKO_SDD_STATUS_ENGRAM` is set (any non-empty value) |
| Engram dir | `.engram/` directory exists at `<cwd>/.engram` |
| Config: engram | `openspec/config.yaml` or `openspec/config.yml` has `artifact_store: engram` or `artifactStore: engram` (case-insensitive value match) |
| Config: hybrid | Same config file has `artifact_store: hybrid` or `artifactStore: hybrid` |

When gating is ON and `selectChange` returns a blocked status, the Engram fallback is
attempted.

---

## REQ-3: Files-first precedence

When a matching OpenSpec change exists on disk (i.e., `selectChange` returns successfully
without a blocked token), the Engram fallback is NOT consulted — even if gating is ON and
Engram holds observations for that change. The file-path result is returned as-is.

---

## REQ-4: Fallback at branch (a) — zero OpenSpec changes, one Engram change

When:
- No change name is requested, AND
- `ListActiveOpenSpecChanges` returns zero changes, AND
- Gating is ON, AND
- Engram observations contain exactly one distinct change name under `sdd/<change>/*`
  for the matched project

Then `Resolve` returns a fully resolved status for that Engram change with:
- `ArtifactStore = "engram"`
- `ChangeRoot = "engram:sdd/<change>"` (not nil)
- `PlanningHome.Path = "engram:sdd"`
- `ChangeName` = `&<change>` (non-nil pointer)
- `NextRecommended` reflecting the artifact state reconstructed from observations
- `BlockedReasons = []` (empty, not nil)

---

## REQ-5: Fallback at branch (b) — named change absent from OpenSpec but present in Engram

When:
- A change name is requested, AND
- That change is not found in `ListActiveOpenSpecChanges`, AND
- Gating is ON, AND
- Engram observations contain at least one `sdd/<requested-change>/*` title for the
  matched project

Then `Resolve` returns a fully resolved status for the requested change from Engram,
with the same origin flags as REQ-4.

---

## REQ-6: Ambiguity — zero OpenSpec changes, multiple Engram changes

When:
- No change name is requested, AND
- `ListActiveOpenSpecChanges` returns zero changes, AND
- Gating is ON, AND
- Engram observations contain two or more distinct change names under `sdd/<change>/*`
  for the matched project

Then `Resolve` returns a blocked status with `NextRecommended = "select-change"` and
a non-empty `BlockedReasons`. The Engram source is indicated (e.g., names listed in
the reason string). No silent auto-selection occurs.

---

## REQ-7: Artifact reconstruction — parity with file path

The artifacts map built from Engram observations MUST follow these rules:

| Key | Present when | State |
|---|---|---|
| `proposal` | A `sdd/<change>/proposal` observation exists with non-empty content | `done` |
| `specs` | A `sdd/<change>/spec` observation exists with non-empty content | `done` |
| `design` | A `sdd/<change>/design` observation exists with non-empty content | `done` |
| `tasks` | A `sdd/<change>/tasks` observation exists with non-empty content | `done` |
| `applyProgress` | A `sdd/<change>/apply-progress` observation exists with non-empty content | `done` |
| `verifyReport` | A `sdd/<change>/verify-report` observation exists with non-empty content | `done` |

A key with no matching observation is `missing`. A key whose observation is present but
has empty (whitespace-only) content is `partial`, mirroring the file path's
`singleArtifactState` (a present-but-empty file is partial). In practice Engram
observations carry non-empty content (`mem_save` requires it), so `partial` is a
defensive state that should not normally occur — it exists so the Engram and file
paths classify artifacts identically.

Task progress (`TaskProgress`) is parsed from the content of the `sdd/<change>/tasks`
observation using the **same** `taskCheckbox` regex (`^\s*(?:[-*]|\d+[.)])\s+\[([ xX])\]`)
that `countTaskProgress` uses for files. Both code paths MUST share a single regex definition.

`NextRecommended` computed from Engram-reconstructed artifacts MUST be identical to
`NextRecommended` that would be computed by the file path for an equivalent artifact state.

---

## REQ-8: Project matching

Observations are matched by project using this inference chain (first match wins):

1. `ENGRAM_PROJECT` environment variable (if set and non-empty)
2. `owner/repo` derived from the `url` field under `[remote "origin"]` in
   `<cwd>/.git/config` (lowercased)
3. Lowercased basename of `cwd`

Only observations where `project` matches the inferred value (case-insensitive) AND
`scope != "personal"` are considered. An observation saved under a different project key
is silently ignored — fail-safe, no false positives.

---

## REQ-9: Degradation safety

When the Engram export path fails for any reason (binary not on PATH, non-zero exit,
malformed JSON, missing required fields in the schema), the fallback returns nothing
and `Resolve` returns the same blocked status (`sdd-new` or `select-change`) that the
file path would have returned. The error is not surfaced to the caller.

Covered failure modes:
- `engram` binary not found on PATH
- `engram export` exits non-zero
- Export output is not valid JSON
- JSON parses but `observations` key is absent or null
- An individual observation lacks `title`, `content`, `project`, or `scope` fields
- No observations match the inferred project

---

## REQ-10: Origin flag is explicit and consumer-safe

An Engram-resolved status MUST always carry:

```
ArtifactStore = "engram"
ChangeRoot    = "engram:sdd/<change>"         // prefixed, not a filesystem path
PlanningHome.Path = "engram:sdd"              // prefixed, not a filesystem path
```

No consumer of `capiko.sdd-status` MUST be required to handle `changeRoot` specially —
the `engram:` prefix exists as an informational signal; routing via `nextRecommended`
continues to work identically.

---

## REQ-11: Contract doc update

`internal/catalog/skills/sdd-shared/sdd-status-contract.md` MUST be updated with a
bounded note clarifying that:

- The engine MAY report `artifactStore: "engram"` when OpenSpec files are absent but
  Engram observations exist.
- `changeRoot` will be prefixed `engram:sdd/<change>` in that case — NOT a filesystem path.
- This is strictly a read-only fallback; the canonical-files invariant is NOT weakened.
- Consumers that parse `changeRoot` as a filesystem path MUST guard against this prefix.

---

## REQ-12: Test seam requirement

The Engram export function (shells out to `engram export <path>`) MUST be exposed as a
package-level function variable (seam) in `internal/sddstatus`, following the same pattern
as `internal/engram`'s `runOut`. Tests swap this seam to inject fixture JSON without
invoking the real `engram` binary. The real binary is NEVER called during `go test -race ./...`.

---

## Acceptance scenarios

All scenarios are testable via the export seam. None shell out to `engram`.

### SC-01: Gating OFF — no behavior change (REQ-1)

```
Given  a workspace with no .engram dir, no openspec/config.yaml, no CAPIKO_SDD_STATUS_ENGRAM
And    no OpenSpec changes exist
When   Resolve is called with no ChangeName
Then   NextRecommended = "sdd-new"
And    BlockedReasons contains "No active OpenSpec changes found under openspec/changes."
And    the export seam is never invoked
```

### SC-02a: Gating ON via env var (REQ-2)

```
Given  CAPIKO_SDD_STATUS_ENGRAM is set to "1"
And    no .engram dir and no openspec config
And    no OpenSpec changes exist
And    the export seam returns one change "my-feature" with proposal done
When   Resolve is called with no ChangeName
Then   the export seam IS invoked
And    the returned status has ArtifactStore = "engram"
```

### SC-02b: Gating ON via .engram dir (REQ-2)

```
Given  a .engram/ directory exists at the workspace root
And    no env var and no openspec config
And    no OpenSpec changes exist
And    the export seam returns one change "my-feature" with proposal done
When   Resolve is called with no ChangeName
Then   the export seam IS invoked
And    ArtifactStore = "engram"
```

### SC-02c: Gating ON via openspec config artifact_store: engram (REQ-2)

```
Given  openspec/config.yaml contains "artifact_store: engram"
And    no .engram dir and no env var
And    no OpenSpec changes exist
And    the export seam returns one change "my-feature"
When   Resolve is called
Then   the export seam IS invoked
```

### SC-02d: Gating ON via openspec config artifact_store: hybrid (REQ-2)

```
Given  openspec/config.yaml contains "artifact_store: hybrid"
And    no .engram dir and no env var
And    no OpenSpec changes exist
And    the export seam returns one change "my-feature"
When   Resolve is called
Then   the export seam IS invoked
```

### SC-03: Files-first — OpenSpec change wins even with gating ON (REQ-3)

```
Given  .engram dir exists (gating ON)
And    an OpenSpec change "my-feature" exists on disk with proposal.md
And    the export seam would return "my-feature" with all artifacts done
When   Resolve is called with no ChangeName
Then   ArtifactStore = "openspec"
And    the export seam is NOT invoked
And    the status reflects the on-disk artifacts, not the Engram observations
```

### SC-04: Fallback branch (a) — zero OpenSpec, one Engram change (REQ-4)

```
Given  gating is ON (.engram dir present)
And    no OpenSpec changes exist
And    the export seam returns observations for exactly one change "add-login":
       - sdd/add-login/proposal  (content: non-empty)
       - sdd/add-login/spec      (content: non-empty)
       - sdd/add-login/design    (content: non-empty)
       - sdd/add-login/tasks     (content: "- [ ] do the thing")
       all with project matching the inferred project, scope != "personal"
When   Resolve is called with no ChangeName
Then   err = nil
And    NextRecommended = "apply"
And    ArtifactStore = "engram"
And    ChangeRoot = "engram:sdd/add-login"
And    PlanningHome.Path = "engram:sdd"
And    ChangeName = "add-login"
And    Artifacts["proposal"] = "done"
And    Artifacts["specs"] = "done"
And    Artifacts["design"] = "done"
And    Artifacts["tasks"] = "done"
And    TaskProgress.Total = 1, Completed = 0, Pending = 1, AllComplete = false
And    BlockedReasons is empty
```

### SC-05: Fallback branch (b) — named change absent from OpenSpec but in Engram (REQ-5)

```
Given  gating is ON (env var set)
And    no OpenSpec changes exist
And    the export seam returns observations for "auth-refactor" with proposal + spec done
When   Resolve is called with ChangeName = "auth-refactor"
Then   err = nil
And    ArtifactStore = "engram"
And    ChangeRoot = "engram:sdd/auth-refactor"
And    ChangeName = "auth-refactor"
And    NextRecommended = "design"
```

### SC-06: Ambiguity — zero OpenSpec, multiple Engram changes (REQ-6)

```
Given  gating is ON (.engram dir present)
And    no OpenSpec changes exist
And    the export seam returns observations for two distinct changes "feat-a" and "feat-b"
       both with project matching the inferred project, scope != "personal"
When   Resolve is called with no ChangeName
Then   NextRecommended = "select-change"
And    BlockedReasons is non-empty (names "feat-a" and "feat-b" appear in the reason)
And    ArtifactStore != "engram" (blocked status retains openspec or empty store)
```

### SC-07a: Task progress parity — checkbox parsing from observation text (REQ-7)

```
Given  the tasks observation content is:
       "- [x] Step 1\n- [ ] Step 2\n- [X] Step 3"
When   countTaskProgressText is called with that string
Then   Total = 3, Completed = 2, Pending = 1, AllComplete = false
And    the result is identical to countTaskProgress called on a file with the same content
```

### SC-07b: nextRecommended parity — Engram vs. file path for equivalent state (REQ-7)

```
Given  an equivalent artifact state (proposal done, spec done, design done, tasks done
       with one unchecked task, no applyProgress, no verifyReport)
When   the state is resolved from Engram observations (via seam)
And    the same state is resolved from OpenSpec files (via the file path)
Then   NextRecommended is equal in both cases
And    Dependencies match in both cases
And    ApplyState matches in both cases
```

### SC-08a: Project matching — env var (REQ-8)

```
Given  ENGRAM_PROJECT = "acme/my-service"
And    the export seam returns observations with project = "acme/my-service" for "my-feature"
And    also observations with project = "other-org/other-repo" for "other-feature"
When   Resolve is called (gating ON, no OpenSpec changes)
Then   only "my-feature" is considered
And    the status resolves "my-feature"
```

### SC-08b: Project matching — git remote (REQ-8)

```
Given  ENGRAM_PROJECT is not set
And    <cwd>/.git/config contains:
       [remote "origin"]
           url = https://github.com/myorg/myrepo.git
And    the export seam returns observations with project = "myorg/myrepo" for "my-feature"
When   Resolve is called (gating ON, no OpenSpec changes)
Then   "my-feature" is matched and the status resolves it
```

### SC-08c: Project matching — dir basename fallback (REQ-8)

```
Given  ENGRAM_PROJECT is not set
And    no .git/config exists (or no origin remote)
And    cwd basename is "MyProject"
And    the export seam returns observations with project = "myproject" for "my-feature"
When   Resolve is called (gating ON, no OpenSpec changes)
Then   "my-feature" is matched (case-insensitive)
```

### SC-08d: Project mismatch — change under different project is not matched (REQ-8)

```
Given  inferred project = "myorg/myrepo"
And    the export seam returns observations only with project = "other/repo" for "my-feature"
When   Resolve is called (gating ON, no OpenSpec changes)
Then   the fallback finds no matching change
And    NextRecommended = "sdd-new" (normal blocked status, no crash)
```

### SC-08e: Personal scope observations are excluded (REQ-8)

```
Given  the export seam returns observations for "my-feature" with the correct project
       but scope = "personal"
When   Resolve is called (gating ON, no OpenSpec changes)
Then   those observations are NOT matched
And    NextRecommended = "sdd-new"
```

### SC-09a: Degradation — engram binary absent (REQ-9)

```
Given  gating is ON
And    no OpenSpec changes exist
And    the export seam returns an error (simulating binary not found)
When   Resolve is called
Then   err = nil
And    NextRecommended = "sdd-new"
And    BlockedReasons contains the normal "No active OpenSpec changes" reason
And    ArtifactStore = "openspec"
```

### SC-09b: Degradation — engram export returns malformed JSON (REQ-9)

```
Given  gating is ON
And    no OpenSpec changes exist
And    the export seam returns (status=0, body="not-json", err=nil)
When   Resolve is called
Then   err = nil
And    NextRecommended = "sdd-new"
And    no panic
```

### SC-09c: Degradation — engram export returns valid JSON with no matching observations (REQ-9)

```
Given  gating is ON
And    no OpenSpec changes exist
And    the export seam returns {"observations":[]} (empty array)
When   Resolve is called with no ChangeName
Then   err = nil
And    NextRecommended = "sdd-new"
```

### SC-10: Origin flags are explicit and consumer-safe (REQ-10)

```
Given  an Engram-resolved status for change "my-feature"
Then   ArtifactStore = "engram"
And    ChangeRoot starts with "engram:sdd/"
And    PlanningHome.Path = "engram:sdd"
And    PlanningHome.Mode = "repo-local" (unchanged)
And    the JSON produced by json.Marshal is valid and parses with all required fields
And    NextRecommended is a valid routing token (not "sdd-new" for a resolved change)
```

### SC-11: Seam isolation — real engram binary is never called in tests (REQ-12)

```
Given  the package-level export seam is set to a fixture function
When   any test in internal/sddstatus runs under go test -race ./...
Then   no test invokes exec.Command("engram", ...)
And    tests complete hermetically without network or PATH dependencies
```

---

## Out of scope

- Engram as a primary artifact store (files remain canonical)
- Phase-skill or agent changes
- New write paths to Engram or OpenSpec
- `engram` binary management (install / upgrade)
- Any change to `SchemaName`, `SchemaVersion`, or the JSON top-level shape
- TUI changes or golden file updates
