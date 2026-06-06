# Strict TDD — Apply Protocol

Load this file when strict TDD is active for the change (the orchestrator forwards
`strict_tdd: true`, or `openspec/config.yaml` sets it). It is the detailed,
non-negotiable protocol for the apply phase. The SKILL.md gives you the one-line
rule; this file is how you actually execute it.

If strict TDD is NOT active, skip this file and use the standard apply flow.

## Philosophy

TDD is not testing — it is design driven by tests. You write the test first
because it forces you to define behavior before implementation, keeps the design
testable, and gives you a safety net for every refactor. A test written after the
code only confirms what you already built; a test written before it shapes what
you build.

### The three laws

1. Write NO production code until you have a failing test that demands it.
2. Write only enough of a test to make it fail (a compile error counts as failing).
3. Write only enough production code to make the failing test pass.

You loop these laws in small steps. No big-bang implementation.

## The cycle

Run this loop per task in `tasks.md`. Do not advance a step until its gate passes.

1. **Safety net** — before touching existing code, confirm the current tests pass.
   If you are modifying behavior that has no test, write a characterization test
   that captures today's behavior FIRST, so you can prove you did not break it.
2. **Understand** — read the task's slice of `spec.md` and `design.md`. Know the
   exact behavior and its acceptance scenario before writing a line.
3. **RED** — write one failing test for the smallest next behavior. Run it. It
   MUST fail, and fail for the RIGHT reason (asserting the missing behavior, not a
   typo or a missing import). A test that passes immediately proves nothing.
4. **GREEN** — write the minimum code to make that test pass. Run it. It MUST pass.
   No extra abstraction, no speculative generality.
5. **Triangulate** — if the spec scenario has more than one case (edge, error,
   boundary), add the next failing test and return to GREEN. Repeat until the
   scenario is fully covered. One example is not a spec.
6. **Refactor** — with tests green, clean up names, duplication, and structure.
   Re-run tests after each change; they stay green or you revert. Refactor is not
   optional polish — it is where the design is paid off.
7. **Done** — task is done only when its tests are green AND the project gate is
   green (see below). Then check the box in `tasks.md`.

## Choosing the test layer

Use the highest-fidelity layer the behavior needs — never skip a task because the
ideal tool is missing; degrade to the next available layer instead.

- **Pure logic / functions** → Go table-driven unit tests (`func TestX(t *testing.T)`
  with a `[]struct` of cases). Prefer pure, side-effect-free functions: they are
  trivial to test and safe to refactor.
- **Bubbletea TUI** → `teatest` golden tests. Drive the model with messages,
  assert the rendered output. Regenerate goldens with
  `go test ./internal/tui -update` and ALWAYS inspect the golden diff before
  committing — an unreviewed golden update is not a test, it is a rubber stamp.
- **Cross-package / filesystem behavior** → integration tests with `t.TempDir()`
  and real file I/O, not mocks of the filesystem.

## Running tests

- During the cycle, run only the package you are working on for fast feedback:
  `go test ./internal/<pkg>`.
- Read the actual test command from `openspec/config.yaml`; do not assume.
- Before marking ANY task done, the full project gate MUST be green:
  `gofmt -l .` (empty), `go vet ./...`, and `go test -race ./...`.
- `-race` is not optional. Concurrency bugs that pass without it are still bugs.

## Assertion quality (mandatory)

A green test that asserts nothing real is worse than no test — it gives false
confidence. These patterns are BANNED:

- **Tautologies** — `if got != got`, asserting a literal equals itself, or
  asserting a value you just assigned without calling production code.
- **No-assert tests** — exercising code and never checking the result. If the only
  thing it proves is "does not panic", say so explicitly and assert that.
- **Type-only checks** — asserting something is non-nil or of a type, when the spec
  demands a specific value.
- **Ghost loops** — ranging over an empty collection so the assertions never run.

A real assertion calls production code, asserts a specific expected output, and
FAILS if the implementation is wrong. Before trusting a green test, ask: "If I
broke the implementation, would this test go red?" If not, the test is theater.

Mock hygiene: more than ~3 mocks in one test file is a smell that the unit is
doing too much or you are testing implementation detail instead of behavior. Do
not assert on internal state, private fields, or mock call counts when you can
assert on observable output.

## Return summary

When you report the phase result, extend the envelope with a TDD evidence block:

```
TDD Evidence:
- Tasks covered: <n>
- Tests added: <n> (unit: <n>, golden: <n>, integration: <n>)
- RED→GREEN confirmed: yes
- Gate: gofmt clean / go vet clean / go test -race pass
```

If you could not follow RED→GREEN for any task, say so explicitly and why — do not
silently fall back to writing code first.

## Rules

- Test-first is non-negotiable. Production code without a preceding failing test is
  a protocol violation, not a shortcut.
- Every RED must be observed failing; every GREEN must be observed passing. "It
  should pass" is not evidence — run it.
- Triangulate multi-scenario specs; one passing case does not satisfy a spec with
  several.
- Protect existing behavior with a safety net before you modify it.
- Prefer pure functions, but do not force purity into contexts where it does not
  fit (I/O, TUI side effects). Isolate the impure edge and test the pure core.
