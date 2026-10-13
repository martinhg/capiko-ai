// Package odd builds the capiko ODD (Organic Driven Development) routing block:
// an always-active implementation-routing protocol that Copilot CLI follows on
// every request. Unlike SDD's delegated-phase pipeline, ODD routes work inline
// or through evidence-budget delegation with no per-phase model assignments.
package odd

import (
	"fmt"
	"strings"
)

const (
	MarkerStart = "<!-- capiko:odd:start -->"
	MarkerEnd   = "<!-- capiko:odd:end -->"
)

const (
	maxEvidenceBatches          = 1
	maxEvidenceCalls            = 3
	approxEvidenceTokens        = 10_000
	approxSequentialLookupLimit = 5
	approxHandoffTokens         = 2_000
	maxParentSpotChecks         = 1
	approxParentContextTokens   = 150_000
)

// Render builds the ODD routing block for injection into Copilot's instructions
// file. When rddEnabled is true, the block includes review-trigger integration
// clauses; otherwise verification uses ordinary functional checks.
func Render(rddEnabled bool) string {
	var b strings.Builder

	b.WriteString("## Implementation Routing\n\n")
	b.WriteString("Organic Driven Development (ODD) is the predefined workflow of this orchestrator. Every request enters it, on every runtime, without the user asking for a workflow, a plan, or task tracking. Never describe this workflow only when asked about it: run it.\n\n")

	renderProtocol(&b, rddEnabled)
	renderTaskSize(&b, rddEnabled)
	renderRoutes(&b)
	renderDelegationTriggers(&b)
	renderODD(&b, rddEnabled)

	return b.String()
}

func renderProtocol(b *strings.Builder, rdd bool) {
	b.WriteString("### ODD protocol (MANDATORY, in this order, on every request)\n\n")
	b.WriteString("1. **Authorize.** First establish whether the requested outcome explicitly authorizes a change. Investigation, explanation, review, audit, comparison, and solution-proposal or planning-only requests are read-only unless the user explicitly requests implementation or another mutation.\n")
	b.WriteString("   - Read-only work may inspect, explain, compare, and recommend, but must not write or edit files, delegate a writer, invoke apply, or create implementation artifacts.\n")
	b.WriteString("   - If change intent is ambiguous or conditional, ask one clarification and remain read-only until answered.\n")
	b.WriteString("2. **Explore.** Explore the existing code and requirements first, proportionately to the request, before proposing or writing anything. Explore only for a map you need to decide or route, never to read files you will read anyway before writing inline.\n")
	b.WriteString("3. **Resolve uncertainty.** Recommend optional research only for a named uncertainty; ask one focused user question only for a real unresolved product decision, then stop and wait; use at most one scoped read-only assumption challenge for a high-consequence unproven premise.\n")
	b.WriteString("4. **Classify.** Size the task by the Task Size section below: small when understood, risk is contained, and the work could be resumed from the original request and `git diff` alone; large only when that resume test fails. Never classify by counting files, commands, tests, fixes, or a requested todo list. Small work stays inline and creates no durable task artifacts.\n")
	b.WriteString("5. **Track before the first write.** For large authorized implementation, create `odd/tasks/<feature-name>.md` and its Engram mirror `odd/<feature-name>/tasks` automatically, before the first source write, without asking permission for tasks or storage. Tell the user in one line which feature document was created and how many tasks it holds. The document is the specification subagents read, in this order: a two- or three-line header; `## Specs` with numbered `S#` that quote the user's exact strings, error messages, and examples verbatim, never summarized and never adding unrequested requirements; `## Tasks` with one line per task (ID, linked `S#`, route, commit); `## Log` last, where `L1` is the user's original request verbatim and later user corrections, evidence, and decisions are appended. A requirement change appends its verbatim Log entry, rewrites only the affected `S#`, and reopens only its task.\n")
	b.WriteString("6. **Implement task by task.** Hand off by reference, never by paraphrase: name the document, task, and specs (for example `Spec: odd/tasks/<feature>.md, T2, S3-S4`), tell workers to read until `## Log`, and ask which `S#` were covered. Without a feature document, include the user's request verbatim. Verify reads the whole document, runs the spec's examples the parent authorized, against isolated state when they mutate data, and returns a verdict per `S#`. When the user reports a failure, reproduce it before deciding it already works. Each test asserts every observable effect of the rule (output, exit code, persisted data), covers the cases the rule itself names, and goes through the public interface. When you add or change a command, option, or message, update the help text and docs. Route each task through the smallest useful topology below, honoring its mandatory delegation triggers, with the default applicable test-first policy and proportional checks. Check an item off only after its outcome and checks were observed; update the file and the mirror after each task. Every tracked task closes with at least one work-unit commit on the feature branch, branch first when on the default branch, with tests and docs alongside the behavior, using a Conventional Commit message; record the commit identity in the feature document as evidence. Work-unit commits on the feature branch are part of authorized large ODD implementation; push, pull request creation, and merge remain the user's decisions under ordinary repository policy.\n")

	closeGate := "Close every code change with its Risk line. Hedged wording is not a stop: a user saying they may stop you or resume later asks for notes and separate commits. A tracked task ends pending only with the user's explicit stop or one **Needs your decision** result naming the open blockers or missing proof; partial, blocked, unavailable, or exhausted proof becomes that one result, never more verification."

	if rdd {
		b.WriteString("7. **Close.** Report the verified outcome, every failed, skipped, or pending check, and the next step. " + closeGate + " After each work-unit commit, evaluate the change against the high-risk items list above. If any item applies, load the applicable review skills and delegate an independent verifier via the Task tool with `include-custom-instructions: true`. Record the risk tier per task.\n\n")
	} else {
		b.WriteString("7. **Close.** Report the verified outcome, every failed, skipped, or pending check, and the next step. " + closeGate + " Verification covers a work-unit commit or a PR slice, never a TODO checkbox and never the accumulated feature branch.\n\n")
	}

	b.WriteString("Resume an interrupted feature with `mem_context`, then project- and feature-scoped `mem_search`, then `mem_get_observation` for the full document, then the task file itself; reconcile before continuing the next unfinished task.\n\n")
}

func renderTaskSize(b *strings.Builder, rdd bool) {
	b.WriteString("### Task Size\n\n")
	b.WriteString("A task is **small** when all three hold:\n\n")
	fmt.Fprintf(b, "1. **Understood**: the outcome is specified, what and where to change is known, and no product or design decision is open; prove it within one bounded read batch (at most %d calls, approximately %dk tokens).\n", maxEvidenceCalls, approxEvidenceTokens/1000)
	b.WriteString("2. **Contained risk**: it touches no high-risk item below.\n")
	b.WriteString("3. **Resumable from the diff**: resume test, if the session stopped now, someone could finish the work from the original request and `git diff` alone.\n\n")
	b.WriteString("The number of files, commands or tests, fixes, or a requested todo list never decides size. A task is **large** only when the resume test fails: several sessions, waiting on something external, separate deliverables, or requirements too long to survive compaction. A large task gets tracking; it gets a writer only by the Writer trigger, otherwise the parent works inline.\n\n")
	b.WriteString("Small path: work inline. Read and edit, several files of one understood change included; observe RED inline before the fix; run the focused test and the suite inline, once each. No explore, worker, or verifier; no feature document, mirror, or commits unless the user asks; a todo list is optional.\n\n")
	b.WriteString("**High risk**: a mistake would be hard to detect, hard to undo, or reaches beyond the change. It touches (1) data or irreversible effects: migrations, changing or deleting existing stored data, persisted format changes (not saving new records); (2) security: auth, permissions, credentials, secrets, guards, sandbox; (3) contracts others consume: changing or removing public API, CLI flags, config formats, published exports, or mirrored prompts or contracts others already consume (not adding a flag, command, or optional field; requested changes are not high risk by themselves; unrequested breaks in shared code are); (4) concurrency: locks, races, async ordering; (5) delivery or environment: installers, release, CI, deploy, dependencies; (6) no test would catch a regression in what changes.")
	if rdd {
		b.WriteString(" When review triggers are active and a trigger returns a tier, that tier wins; you may only raise it.")
	}
	b.WriteString("\n\n")
	b.WriteString("**Risk line**: close every code change, small path or delegated, with `Risk: item N (reason)` or `Risk: none` per the list; any item fires the Verification trigger. Before writing `Risk: none`, check whether the diff changes code that existing behavior the request did not mention also uses (shared options, parsers, helpers); if it does, that is item 3.\n\n")
	b.WriteString("After explicit change intent is established, route work for the requested outcome with the smallest useful topology. Every authorized change takes exactly one implementation route: direct inline or delegated direct.\n\n")
}

func renderRoutes(b *strings.Builder) {
	fmt.Fprintf(b,
		"- **Direct inline:** small tasks, by the Task Size section, and large tasks without a Writer trigger reason, which follow their logbook. Decide or verify with one parallel batch (%d maximum), at most %d calls and approximately %dk tokens of evidence. Use bounded search/line ranges, not whole large files. One understood change stays inline even when it spans files, and its focused test and suite run inline.\n",
		maxEvidenceBatches, maxEvidenceCalls, approxEvidenceTokens/1000,
	)
	fmt.Fprintf(b,
		"- **Delegated direct:** understanding that needs more evidence or more than approximately %d sequential lookups takes one read-only explorer, then task size is re-evaluated; a writer is delegated only by the Writer trigger.\n",
		approxSequentialLookupLimit,
	)
	b.WriteString("- File count, changed lines, size, or perceived risk alone never forces a heavier route.\n")
	b.WriteString("- These are implementation routes, not a ban on per-action delegation. Tests, builds, installs, and verification actors may still use fresh workers via the Task tool without changing the selected route.\n")
	b.WriteString("- When delegating any worker, set `include-custom-instructions: true` so the worker inherits the project's instruction files.\n")
}

func renderDelegationTriggers(b *strings.Builder) {
	b.WriteString("\n### Mandatory Delegation Triggers\n\n")
	b.WriteString("These triggers are mandatory, not advisory. Each mechanism turns on only by its own trigger. When one fires, stop and delegate through the Task tool before continuing; executing past a fired trigger inline is a routing defect even if the work succeeds. When it resolves, re-evaluate task size and stay small while the three criteria hold. Delegation keeps the parent context thin enough to orchestrate; it does not slow the work down.\n\n")
	b.WriteString("- **Ask trigger:** an open product or design decision gets one focused user question; stop and wait.\n")
	fmt.Fprintf(b, "- **Mapping trigger:** when understanding exceeds the inline batch budget or needs more than approximately %d sequential lookups, delegate one read-only explorer before deciding or writing anything. Return a handoff of at most approximately %dk tokens with path:line evidence; use one parent spot check (%d maximum). Do not reread the entire mapped evidence. Never explore to read files you will read anyway before writing inline.\n", approxSequentialLookupLimit, approxHandoffTokens/1000, maxParentSpotChecks)
	b.WriteString("- **Verification trigger:** a high-risk change gets an independent verifier after the change's own checks; otherwise whoever made the change runs its focused test and suite inline, small tasks included.\n")
	b.WriteString("- **Tracking trigger:** a large task gets the feature document, its mirror, and work-unit commits.\n")
	b.WriteString("- **Writer trigger:** a writer is delegated only for a named reason, one bounded writer per unit: (a) parallelism: two or more independent units with disjoint edit surfaces, each clearly heavier than starting a subagent, launched together; (b) context: the long-session backstop below. Never delegate a writer for size, a large task alone, file count, or a price ratio; file count never fires this trigger. Without a reason, the parent works inline, following its logbook (feature document, mirror, work-unit commits).\n")
	b.WriteString("- **Parallel writers:** work in another repository goes in a fresh worktree based on that repository's main, never in a checkout someone else may be using. Parallel units in the same local repository run in the same tree, each with declared disjoint edit surfaces: the parent owns git; writers run no repository-wide formatters, generators, or installs; a shared file has one owner or the parent edits it at the end; the parent checks each writer's diff against its surface before accepting it; and one seam check runs at the end (the full suite, once). Units that need the same file use isolated worktrees.\n")
	b.WriteString("- **Preparation trigger:** when the Writer trigger delegates a write, reading that prepares it, and broad research or context compression, delegate together with or ahead of the write instead of filling the parent context. Inline writes read inline.\n")
	b.WriteString("- **Output budget:** keep parent bash output bounded to counts, --stat, tail, or summaries. On a large task, delegate long suites and builds and return concise observed results, including failures.\n")
	fmt.Fprintf(b, "- **Long-session backstop:** at approximately %dk parent-context tokens, pause and delegate the next bounded unit. This is advisory context guidance, not mechanically observed or enforced; do not claim runtime telemetry or enforcement.\n", approxParentContextTokens/1000)
	b.WriteString("- **Route declaration:** for large work, record the chosen route per task (inline or delegated) and the trigger evidence in the feature document, so skipped delegation is observable instead of silent.\n")
	b.WriteString("- These triggers only choose between direct inline and delegated direct inside the organic flow.\n")
}

func renderODD(b *strings.Builder, rdd bool) {
	b.WriteString("\n### Organic Driven Development\n\n")
	b.WriteString("Use this flow for direct and delegated work. Explore the existing code and requirements first, proportionately to the request; the mutation-authorization guard above still applies.\n\n")
	b.WriteString("This section is the reference detail for the protocol above.\n\n")
	b.WriteString("- Recommend optional research only for a named uncertainty. If declined, continue within authorized scope only where safe without the missing evidence; disclose unresolved uncertainty and pause affected unsafe decisions. Offer a concise proposal only when a real scope or product decision needs it. Neither research nor a proposal is mandatory.\n")
	b.WriteString("- Establish the problem, intended outcome, constraints, and current evidence; inspect relevant code. Adapt depth to uncertainty and consequence, not a fixed questionnaire or mandatory rounds. The parent owns product decisions: ask one focused user question only for a real unresolved product decision, then stop and wait. Workers return gaps to the parent rather than assuming choices.\n")
	b.WriteString("- When the question needs external evidence, use available authorized documentation/web tools and prefer primary sources. Attribute material claims to source URLs or code locations; distinguish verified facts, assumptions, contradictions, freshness, and gaps. If tools are unavailable, disclose limitations without inventing access or evidence; pause only unsafe decisions dependent on missing evidence.\n")
	b.WriteString("- Return concise findings, recommendation, tradeoffs, open questions, and implementation implications. When delegating research, forward these research instructions to a fresh general exploration/research worker through the Task tool; do not create a specialized research agent. Keep research read-only; its findings do not authorize implementation or require new persistence or readiness machinery.\n")
	b.WriteString("- Use at most one scoped independent read-only assumption challenge for a high-consequence unproven premise, even in a small security-critical change. Name the premise, evidence, and consequence; do not start a debate loop. Deterministic failures need fixes, not model debate.\n")

	if rdd {
		b.WriteString("- Validate consequential premises against available evidence before building; reuse relevant sibling investigation instead of repeating it. Run focused checks during iteration and all applicable full checks at task closure. Keep spend proportional without a hard spend or line gate; preserve the applicable test-first policy, review trigger rules, safety, and consent requirements.\n")
	} else {
		b.WriteString("- Validate consequential premises against available evidence before building; reuse relevant sibling investigation instead of repeating it. Run focused checks during iteration and all applicable full checks at task closure. Keep spend proportional without a hard spend or line gate; preserve the applicable test-first policy and safety requirements.\n")
	}

	b.WriteString("- Size every task by the Task Size section; small work creates no durable task artifacts, and counts of files, commands, tests, fixes, or lines never make it large. For large authorized implementation, automatically create the feature document after exploration, without a task or storage permission prompt.\n")

	taskSizeNonTriggers := "hard cap, counter-trigger, automatic stop, or forced split"
	b.WriteString("- Use about 400 authored changed lines per ODD task only as a planning heuristic, counting additions plus deletions; prefer the smallest coherent behavior with its tests and docs. This is not a task acceptance criterion, " + taskSizeNonTriggers + ". If the correct, clear solution naturally exceeds it, briefly explain why and continue without size-only rework loops. Never delete spaces, blank lines, or comments for cosmetic line savings; never omit tests, minify, add gratuitous abstractions, or split artificially to fit the heuristic. Forward this same advisory-only instruction when delegating tasks to subagents. The delivery budget below reads the accumulated branch, not this per-task heuristic. Existing PR size gates remain unchanged; continue under existing repository policy.\n")

	b.WriteString("- Keep `odd/tasks/<feature-name>.md` and an Engram recovery copy under topic `odd/<feature-name>/tasks`, scoped to the current project. Use a descriptive filename-safe feature name. Reuse the same feature identity; never overwrite another feature. Keep one feature document, not a separate plan file or topic. It is the specification subagents read by reference, so keep this fixed order, stable content first and the growing log last: a header of two or three lines with the objective, branch, delivery strategy, and test runner; `## Specs` with numbered `S1`..`Sn` requirements and their authorized scope, acceptance criteria, and applicable checks, quoting the user's exact strings, error messages, commands, and examples verbatim in the user's language (number and order them, but never summarize or reword those fragments; Do not add requirements the user never asked for, and mark assumptions as assumptions); `## Tasks` with one line per task, using stable task IDs, with its linked `S#`, route (inline or delegated), status, and commit identity or RED/GREEN evidence; `## Log` as the logbook, where `L1` holds the user's original request verbatim, later user corrections are logged verbatim with their date, and problem analysis, rationale for meaningful accepted changes, verification evidence, progress, and next step are appended. Routine corrections stay brief; no exhaustive decision journal. Mirror the full current document and repository-relative file locator, not just a summary or completion notice.\n")

	b.WriteString("- Accepted user, review, or verification changes automatically update affected intent and TODOs, unlike a process that regenerates every artifact: a requirement change appends its verbatim `## Log` entry, rewrites only the affected `S#`, and reopens only its linked task; preserve valid completed and unrelated work; add genuinely new tasks or reopen invalidated items with a reason, and revise their checks. Findings alone never authorize scope expansion or automatic acceptance. Business scope changes still require user authorization. Check off only observed outcomes with applicable proof; record failed, unavailable, skipped, or pending checks honestly. Checkboxes grant no approval.\n")

	b.WriteString("- Read back both writes; they are not atomic. If Engram is unavailable, preserve local progress and explicitly mark the mirror pending; do not claim success or block unrelated safe work. Resynchronize when available. If a file write is unsafe or unavailable, preserve existing state and report the limitation. Preserve both versions on irreconcilable edits and ask only about the real conflict.\n")
	b.WriteString("- On resume, use `mem_context`, then `mem_search` scoped to the current project and feature, and `mem_get_observation` for the full saved document; read the actual task file. Do not infer active work from the newest global memory. Reconcile current requirements, code, and proof before resuming the next unfinished task; preserve pending mirrors and conflicting edits.\n")
	b.WriteString("- Before implementation or resume, the parent reads both the actual file and full observation, reconciles them, and passes the locator, task IDs, and linked `S#`; workers read the document until `## Log` before edits and report which `S#` they covered. Small work without a document still receives its authorized scope and checks.\n")

	if rdd {
		b.WriteString("- Preserve applicable functional verification via capiko's review trigger rules. Run applicable functional checks per task, not a review cycle per TODO checkbox. Verify each work-unit commit in proportion to its risk: after each work-unit commit, evaluate the change against the high-risk items list. If any item applies, load the applicable review skills and delegate an independent verifier via the Task tool. Record per task the risk tier and the checks observed. Never infer low risk without evidence; count an unclear change as high only when a bounded look cannot tell whether the list applies. Never skip an existing delivery gate. A task list or assumption challenge never replaces verification or adds an execution harness.\n")
	} else {
		b.WriteString("- Preserve applicable functional verification. Run applicable functional checks per task, not a verification ceremony per TODO checkbox. Verify each work-unit commit in proportion to its risk, as the orchestrator's Delegated Verification Gate describes: structural readback for passive changes, writer self-verification for ordinary changes, and an added independent verifier for high-risk changes by the Task Size list. Record per task the risk tier you applied and the checks you observed. Never infer low risk without evidence; count an unclear change as high only when a bounded look cannot tell whether the list applies. Never skip an existing delivery gate. A task list or assumption challenge never replaces verification or adds an execution harness.\n")
	}

	b.WriteString("- Apply one default test-first policy to ODD behavior changes: when a relevant runnable deterministic test and clear expected outcome exist, observe RED before implementation, implement GREEN, then refactor while tests stay green. Write one RED test per requested rule. For every existing command or option the change touches, add one test proving its previous behavior still holds; add no other cases. An existing behavior counts as touched when it shares the code you changed (options, parsers, helpers, validation). Tests or frameworks being present alone do not establish applicability. For passive documentation, unavailable runners, or no meaningful runnable RED, explain the exception and run proportionate functional or structural checks. Forward the applicable runner, this test discipline, and the evidence or exception to workers; never invent RED/GREEN evidence or a runner.\n")

	b.WriteString("- Delivery follows work units. At feature-document creation, forecast authored changed lines (additions plus deletions, generated files excluded) from the task list, and keep a running count from work-unit commits. Choose one delivery strategy per feature: `ask-on-risk` (default), `auto-chain`, `single-pr`, or `exception-ok`. When the forecast or the running count exceeds about 400 authored changed lines, apply the chosen strategy before the next commit: `ask-on-risk` asks once for the chain strategy, `stacked-to-main` or `feature-branch-chain`; `auto-chain` asks only for a missing chain strategy and slices automatically. Cache both choices, and record slice boundaries, which commits each pull request holds, in the feature document. Resolve the `work-unit-commits` and `chained-pr` skills by registry name before planning or creating any pull request, never hardcode their paths.\n")
}
