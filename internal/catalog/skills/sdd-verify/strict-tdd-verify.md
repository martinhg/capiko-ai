# Strict TDD — Verify Protocol

Load this file when strict TDD is active for the change (the orchestrator forwards
`strict_tdd: true`, or `openspec/config.yaml` sets it). It is the detailed protocol
for auditing TDD compliance during the verify phase. The SKILL.md tells you to
verify the contract; this file tells you how to verify the change was actually
built test-first, not tested after the fact.

If strict TDD is NOT active, skip this file.

## Philosophy

Standard verification answers "does it work?". Strict-TDD verification also answers
"was TDD followed?". A change can pass its tests and still violate the protocol —
tests written after the code, theater assertions, or untriangulated scenarios. You
audit the evidence, you do not take the apply phase's word for it. You only report
findings; the orchestrator decides remediation. Do NOT fix code here.

## TDD compliance check

1. **Tests exist** — for every behavior the change adds, a test file must exist in
   the tree. Claimed-but-absent tests are **CRITICAL**.
2. **Tests pass when YOU run them** — run the test command from
   `openspec/config.yaml` (e.g. `go test -race ./...`) yourself. A test the apply
   phase reported passing that fails now is **CRITICAL**. Never trust the report;
   cross-reference it against execution.
3. **Triangulation** — count the test cases against the spec scenarios. A spec with
   several scenarios (edge, error, boundary) covered by a single case is
   **WARNING** — the spec was not fully exercised.
4. **Safety net** — if existing files were modified, a characterization test should
   have protected them before the change. Modified files claiming "no test needed"
   are **WARNING** (post-hoc coverage on existing behavior).

## Test layer check

Classify each test by layer and confirm the depth fits the behavior:

- Pure logic → Go table-driven unit tests.
- Bubbletea TUI → `teatest` golden tests. A golden that was regenerated with
  `-update` but never inspected is not real verification — if the diff looks
  unreviewed, flag **WARNING**.
- Cross-package / filesystem → integration tests with `t.TempDir()`.

Critical logic covered by only one shallow layer is a **SUGGESTION** to add depth.

## Assertion quality audit (mandatory)

This is where theater tests are caught. Scan changed test files for banned
patterns:

- **Tautology** — `if got != got`, asserting a literal against itself, asserting a
  value you just assigned without calling production code → **CRITICAL** (proves
  nothing).
- **No production call** — assertions that never invoke the implementation under
  test → **CRITICAL**.
- **Ghost loop** — assertions inside a `range` over a collection that can be empty;
  if it is empty the assertions never run → **CRITICAL**.
- **Incomplete cycle** — a test that passes only because a precondition stops the
  code path from running → **CRITICAL** (the exercised path must actually run).
- **Type-only assertion** — `!= nil` / type check alone when the spec demands a
  specific value → **WARNING**.
- **Orphan empty assertion** — asserting an empty result with no companion
  non-empty case → **WARNING**.
- **Implementation-detail coupling** — asserting internal state, private fields, or
  mock call counts instead of observable behavior → **WARNING**.
- **Mock-heavy** — far more mocks than real assertions in a file → **WARNING**
  (extract a pure function or move up a layer).

For each green test ask: "If the implementation were broken, would this go red?" If
not, it is theater — flag it.

## Changed-file coverage

Run coverage and filter to the files this change created or modified (get the file
list from apply-progress): `go test -cover ./<changed-pkgs>` or
`go test -coverprofile` then inspect. Coverage below ~80% on a changed file is a
**WARNING** with the specific uncovered lines named — not just a percentage.
Coverage and quality metrics are informational; they are never CRITICAL on their
own.

## Quality gate

Confirm the project gate is green on the change: `gofmt -l .` (empty), `go vet
./...`, `go test -race ./...`. A failing gate is **CRITICAL**; vet warnings on
changed code are **WARNING**.

## Severity mapping

| Finding                                   | Severity   |
| ----------------------------------------- | ---------- |
| Claimed test absent / fails when run      | CRITICAL   |
| Tautology / no-production-call / ghost loop | CRITICAL |
| Incomplete TDD cycle                      | CRITICAL   |
| Failing project gate                      | CRITICAL   |
| Untriangulated multi-scenario spec        | WARNING    |
| Type-only / orphan-empty assertion        | WARNING    |
| Implementation-detail coupling            | WARNING    |
| Mock-heavy test                           | WARNING    |
| Low coverage on changed files             | WARNING    |
| Single shallow layer on critical logic    | SUGGESTION |

## Rules

- Do not trust the apply report — cross-reference every claim against actual
  execution.
- The assertion audit is mandatory; a trivial green test is worse than a missing
  one because it hides the gap.
- CRITICAL requires a real defect (absent/failing test, theater assertion, broken
  gate). Missing tools or low coverage are WARNING at most.
- Only report. The orchestrator decides what gets fixed.
