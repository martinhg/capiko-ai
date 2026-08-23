package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/martinhg/capiko-ai/internal/rdd"
	"github.com/martinhg/capiko-ai/internal/reviewstore"
)

// reviewUsage is the usage block printed for a missing or unknown
// subcommand/verb.
const reviewUsage = "Usage:\n" +
	"  capiko-ai review mode enable\n" +
	"  capiko-ai review mode disable\n" +
	"  capiko-ai review mode status\n" +
	"  capiko-ai review start --consent relay|granted|declined\n" +
	"  capiko-ai review capture-result --lens <id> <result-file>\n" +
	"  capiko-ai review finalize\n" +
	"  capiko-ai review schema reviewer"

// resolveWorkspace resolves the current workspace root for RDD kill-switch
// scoping, defaulting to the process's current working directory.
// Package-level var seam (design "cmd/capiko-ai | resolveWorkspace") so
// tests can stub it without depending on the real cwd.
var resolveWorkspace = func() (string, error) {
	return os.Getwd()
}

// userHomeDirFn resolves the user's home directory, used to locate the
// global-scope kill-switch record at ~/.capiko/review-mode.json.
// Package-level var seam (design "cmd/capiko-ai | userHomeDirFn").
var userHomeDirFn = os.UserHomeDir

// reviewNow is a seam over time.Now for deterministic ModeRecord.UpdatedAt
// timestamps in tests, mirroring cga.go's cgaNow.
var reviewNow = time.Now

// buildCandidate resolves a frozen CandidateIdentity for workspace together
// with the raw changed-path list rdd.Classify needs (BuildIdentity only
// exposes a stable digest over paths+modes, not the paths themselves — see
// reviewstore.ChangedPaths). Package-level var seam so `review start` and
// `review capture-result` tests never need a real git repository.
var buildCandidate = func(workspace string) (rdd.CandidateIdentity, []string, error) {
	identity, err := reviewstore.BuildIdentity(workspace)
	if err != nil {
		return rdd.CandidateIdentity{}, nil, err
	}
	changedPaths, err := reviewstore.ChangedPaths(workspace, identity.BaseTree, identity.CandidateTree)
	if err != nil {
		return rdd.CandidateIdentity{}, nil, err
	}
	return identity, changedPaths, nil
}

// readFile is a seam over os.ReadFile for reading a `review capture-result`
// result file, so tests can stub read failures without needing real files.
var readFile = os.ReadFile

// reviewAuthorityDir resolves the CAS-protected authority store directory
// under commonDir (design "Storage Layout": <git-common-dir>/capiko/rdd/),
// shared by review-state.json and review-receipt.json.
func reviewAuthorityDir(commonDir string) string {
	return filepath.Join(commonDir, "capiko", "rdd")
}

// computeSubjectHash derives a stable digest that binds a frozen candidate
// identity, so `review capture-result` can detect workspace drift between
// `review start` and each lens submission (spec review-cli-commands: "Bind
// to the frozen subject hash").
func computeSubjectHash(identity rdd.CandidateIdentity) string {
	h := sha256.New()
	h.Write([]byte(identity.RepositoryID))
	h.Write([]byte{0})
	h.Write([]byte(identity.BaseTree))
	h.Write([]byte{0})
	h.Write([]byte(identity.CandidateTree))
	h.Write([]byte{0})
	h.Write([]byte(identity.ChangedPathsModesDigest))
	h.Write([]byte{0})
	h.Write([]byte(identity.PolicyHash))
	return hex.EncodeToString(h.Sum(nil))
}

// extractFlagValue scans args for flag followed by its value, returning the
// value and the remaining args with both removed. It fails closed: a
// missing flag or a flag with no following value is an error rather than an
// empty-string default (spec review-consent: "Flag required").
func extractFlagValue(args []string, flag string) (value string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	found := false
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("%s flag requires a value", flag)
			}
			value = args[i+1]
			found = true
			i++
			continue
		}
		rest = append(rest, args[i])
	}
	if !found || value == "" {
		return "", nil, fmt.Errorf("%s flag required", flag)
	}
	return value, rest, nil
}

// reviewCommand handles headless RDD kill-switch operations.
//
//	capiko-ai review mode enable|disable|status
func reviewCommand(name string, args []string, out io.Writer) (handled bool, exitCode int, err error) {
	if name != "review" {
		return false, 0, nil
	}

	if len(args) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review: subcommand required (mode)")
	}

	sub := args[0]
	switch sub {
	case "mode":
		return reviewMode(args[1:], out)
	case "start":
		return reviewStart(args[1:], out)
	case "capture-result":
		return reviewCaptureResult(args[1:], out)
	case "finalize":
		return reviewFinalize(args[1:], out)
	case "schema":
		return reviewSchema(args[1:], out)
	default:
		fmt.Fprintf(out, "review: unknown subcommand %q\n", sub)
		fmt.Fprintln(out, reviewUsage)
		return true, 1, nil
	}
}

// reviewMode dispatches `review mode <verb>` to enable, disable, or status.
func reviewMode(args []string, out io.Writer) (bool, int, error) {
	if len(args) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review mode: verb required (enable, disable, status)")
	}

	verb := args[0]
	switch verb {
	case "enable":
		return reviewModeSet(rdd.ModeManaged, out)
	case "disable":
		return reviewModeSet(rdd.ModeDisabled, out)
	case "status":
		return reviewModeStatus(out)
	default:
		fmt.Fprintf(out, "review: unknown mode verb %q\n", verb)
		fmt.Fprintln(out, reviewUsage)
		return true, 1, nil
	}
}

// cloneModePath resolves the clone-scope kill-switch record path under
// commonDir (design "Storage Layout": <git-common-dir>/capiko/rdd/review-mode.json).
func cloneModePath(commonDir string) string {
	return filepath.Join(commonDir, "capiko", "rdd", "review-mode.json")
}

// globalModePath resolves the global-scope kill-switch record path under
// the user's home directory (design "Storage Layout": ~/.capiko/review-mode.json).
func globalModePath(home string) string {
	return filepath.Join(home, ".capiko", "review-mode.json")
}

// reviewModeSet persists mode as the clone-scope kill-switch record for the
// current workspace (spec rdd-review-cli; design "CLI Command Design").
// Enabling and disabling only ever touch the clone-scope record — never the
// global scope, and never any authority state or receipt (spec
// rdd-kill-switch "No fabrication, no destruction").
func reviewModeSet(mode rdd.ReviewMode, out io.Writer) (bool, int, error) {
	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving workspace: %w", err)
	}

	commonDir, err := reviewstore.GitCommonDir(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving git common dir: %w", err)
	}

	clonePath := cloneModePath(commonDir)
	rec := &rdd.ModeRecord{
		SchemaVersion: 1,
		Mode:          mode,
		UpdatedAt:     reviewNow().UTC().Format(time.RFC3339),
	}
	if err := reviewstore.SaveModeRecord(clonePath, rec); err != nil {
		return true, 1, fmt.Errorf("review: saving clone mode record: %w", err)
	}

	fmt.Fprintf(out, "review mode set to %s (clone scope: %s)\n", mode, clonePath)
	return true, 0, nil
}

// reviewModeStatus prints the effective (precedence-resolved) kill-switch
// mode alongside the clone-scope and global-scope records and the git
// common directory (spec rdd-review-cli "Status reports effective mode";
// design "CLI Command Design": "prints: effective mode, clone-scope mode
// (or "not set"), global-scope mode (or "not set"), and the git common
// directory path"). A corrupt record at either scope is reported, not
// fatal — it resolves as absent, so the effective mode still fails closed
// to managed (spec rdd-kill-switch "Fail-closed default").
func reviewModeStatus(out io.Writer) (bool, int, error) {
	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving workspace: %w", err)
	}

	commonDir, err := reviewstore.GitCommonDir(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving git common dir: %w", err)
	}

	home, err := userHomeDirFn()
	if err != nil {
		return true, 1, fmt.Errorf("review: resolving home dir: %w", err)
	}

	clonePath := cloneModePath(commonDir)
	cloneRec, cloneErr := reviewstore.LoadModeRecord(clonePath)
	if cloneErr != nil && !errors.Is(cloneErr, reviewstore.ErrCorruptMode) {
		return true, 1, fmt.Errorf("review: loading clone mode record: %w", cloneErr)
	}

	globalPath := globalModePath(home)
	globalRec, globalErr := reviewstore.LoadModeRecord(globalPath)
	if globalErr != nil && !errors.Is(globalErr, reviewstore.ErrCorruptMode) {
		return true, 1, fmt.Errorf("review: loading global mode record: %w", globalErr)
	}

	// Corrupt records resolve as absent at that scope — same fail-closed
	// treatment as a missing file (design "Corruption fails closed").
	effectiveClone := cloneRec
	if cloneErr != nil {
		effectiveClone = nil
	}
	effectiveGlobal := globalRec
	if globalErr != nil {
		effectiveGlobal = nil
	}
	effective := rdd.ResolveMode(effectiveGlobal, effectiveClone)

	fmt.Fprintf(out, "effective mode: %s\n", effective)
	fmt.Fprintf(out, "clone mode:     %s\n", formatModeRecord(cloneRec, cloneErr))
	fmt.Fprintf(out, "global mode:    %s\n", formatModeRecord(globalRec, globalErr))
	fmt.Fprintf(out, "git common dir: %s\n", commonDir)
	return true, 0, nil
}

// formatModeRecord renders a mode record (or its absence/corruption) as a
// short human-readable status word.
func formatModeRecord(rec *rdd.ModeRecord, err error) string {
	if err != nil {
		return fmt.Sprintf("corrupt (%v)", err)
	}
	if rec == nil {
		return "not set"
	}
	return string(rec.Mode)
}

// resolveEffectiveMode loads the clone- and global-scope kill-switch
// records for commonDir/home and returns the precedence-resolved effective
// mode. Corrupt records resolve as absent at that scope — same fail-closed
// treatment reviewModeStatus applies (design "Corruption fails closed").
func resolveEffectiveMode(commonDir, home string) (rdd.ReviewMode, error) {
	clonePath := cloneModePath(commonDir)
	cloneRec, cloneErr := reviewstore.LoadModeRecord(clonePath)
	if cloneErr != nil && !errors.Is(cloneErr, reviewstore.ErrCorruptMode) {
		return "", fmt.Errorf("loading clone mode record: %w", cloneErr)
	}
	if cloneErr != nil {
		cloneRec = nil
	}

	globalPath := globalModePath(home)
	globalRec, globalErr := reviewstore.LoadModeRecord(globalPath)
	if globalErr != nil && !errors.Is(globalErr, reviewstore.ErrCorruptMode) {
		return "", fmt.Errorf("loading global mode record: %w", globalErr)
	}
	if globalErr != nil {
		globalRec = nil
	}

	return rdd.ResolveMode(globalRec, cloneRec), nil
}

// lensSelected reports whether lens is present in selected.
func lensSelected(selected []rdd.Lens, lens rdd.Lens) bool {
	for _, l := range selected {
		if l == lens {
			return true
		}
	}
	return false
}

// allMandatedCaptured reports whether results has an entry for every lens in
// selected.
func allMandatedCaptured(selected []rdd.Lens, results map[rdd.Lens]reviewstore.LensResult) bool {
	for _, l := range selected {
		if _, ok := results[l]; !ok {
			return false
		}
	}
	return true
}

// reviewStart handles `review start --consent relay|granted|declined`
// (spec review-cli-commands, review-consent; design "Data Flow"):
// resolves the workspace, requires the kill switch to be managed, builds
// and classifies the candidate identity, selects mandated lenses, then
// branches on --consent: relay outputs a ConsentResult envelope without
// freezing anything; declined fails closed without freezing or
// transitioning; granted freezes the candidate, selected lenses, and
// consent evidence into review-state.json and transitions
// unreviewed -> reviewing.
func reviewStart(args []string, out io.Writer) (bool, int, error) {
	consentValue, _, err := extractFlagValue(args, "--consent")
	if err != nil {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review start: %w", err)
	}
	consent, err := rdd.ParseConsentFlag(consentValue)
	if err != nil {
		return true, 1, fmt.Errorf("review start: %w", err)
	}

	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review start: resolving workspace: %w", err)
	}

	commonDir, err := reviewstore.GitCommonDir(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review start: resolving git common dir: %w", err)
	}

	home, err := userHomeDirFn()
	if err != nil {
		return true, 1, fmt.Errorf("review start: resolving home dir: %w", err)
	}

	effective, err := resolveEffectiveMode(commonDir, home)
	if err != nil {
		return true, 1, fmt.Errorf("review start: %w", err)
	}
	if effective != rdd.ModeManaged {
		return true, 1, fmt.Errorf("review start: review mode is %q, not managed; run `review mode enable` first", effective)
	}

	identity, changedPaths, err := buildCandidate(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review start: building candidate identity: %w", err)
	}
	classification := rdd.Classify(changedPaths)
	lenses := rdd.SelectLenses(classification.Tier)

	if consent == rdd.ConsentDeclined {
		return true, 1, fmt.Errorf("review start: consent declined; review not started")
	}

	if consent == rdd.ConsentRelay {
		envelope := rdd.ConsentResult{
			Headline: "Consent required to start review",
			Reason: fmt.Sprintf(
				"Candidate classified at risk tier %d; %d lens(es) mandated",
				classification.Tier, classification.Lenses,
			),
			Evidence: strings.Join(classification.Reasons, "; "),
			Granted:  false,
		}
		data, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return true, 1, fmt.Errorf("review start: encoding consent result: %w", err)
		}
		fmt.Fprintln(out, string(data))
		return true, 0, nil
	}

	// consent == granted
	authorityDir := reviewAuthorityDir(commonDir)
	store := reviewstore.NewStore(authorityDir)
	current, err := store.LoadState()
	if err != nil {
		return true, 1, fmt.Errorf("review start: loading review state: %w", err)
	}

	currentState := rdd.StateUnreviewed
	version := 0
	createdAt := reviewNow().UTC().Format(time.RFC3339)
	if current != nil {
		currentState = current.State
		version = current.Version
		if current.CreatedAt != "" {
			createdAt = current.CreatedAt
		}
	}

	if err := rdd.ValidateTransition(currentState, rdd.StateReviewing); err != nil {
		return true, 1, fmt.Errorf("review start: %w", err)
	}

	consentResult := rdd.ConsentResult{
		Headline: "Consent granted",
		Reason: fmt.Sprintf(
			"Candidate classified at risk tier %d; %d lens(es) mandated",
			classification.Tier, classification.Lenses,
		),
		Evidence: strings.Join(classification.Reasons, "; "),
		Granted:  true,
	}

	newState := &reviewstore.ReviewState{
		SchemaVersion:  1,
		Version:        version,
		Candidate:      identity,
		Tier:           classification.Tier,
		Lenses:         classification.Lenses,
		State:          rdd.StateReviewing,
		SubjectHash:    computeSubjectHash(identity),
		SelectedLenses: lenses,
		Consent:        &consentResult,
		CreatedAt:      createdAt,
		UpdatedAt:      reviewNow().UTC().Format(time.RFC3339),
	}

	if err := store.SaveState(newState); err != nil {
		return true, 1, fmt.Errorf("review start: saving review state: %w", err)
	}

	fmt.Fprintf(out, "review started: tier=%d lenses=%d subject_hash=%s state=%s\n",
		classification.Tier, classification.Lenses, newState.SubjectHash, newState.State)
	return true, 0, nil
}

// reviewCaptureResult handles
// `review capture-result --lens <id> <result-file>` (spec
// review-cli-commands; design "Data Flow"): loads the current review state,
// requires it to be reviewing, requires the lens to be mandated and not yet
// captured, requires the workspace to still match the frozen subject hash,
// then stores the lens's findings. When every mandated lens has been
// captured, it transitions reviewing -> findings_frozen.
func reviewCaptureResult(args []string, out io.Writer) (bool, int, error) {
	lensValue, rest, err := extractFlagValue(args, "--lens")
	if err != nil {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review capture-result: %w", err)
	}
	if len(rest) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review capture-result: result file path required")
	}
	filePath := rest[0]

	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review capture-result: resolving workspace: %w", err)
	}

	commonDir, err := reviewstore.GitCommonDir(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review capture-result: resolving git common dir: %w", err)
	}

	store := reviewstore.NewStore(reviewAuthorityDir(commonDir))
	current, err := store.LoadState()
	if err != nil {
		return true, 1, fmt.Errorf("review capture-result: loading review state: %w", err)
	}
	if current == nil || current.State != rdd.StateReviewing {
		state := rdd.StateUnreviewed
		if current != nil {
			state = current.State
		}
		return true, 1, fmt.Errorf(
			"review capture-result: review state is %q, want %q (run `review start` first)",
			state, rdd.StateReviewing,
		)
	}

	lens := rdd.Lens(lensValue)
	if _, err := rdd.LookupMandate(lens); err != nil {
		return true, 1, fmt.Errorf("review capture-result: %w", err)
	}
	if !lensSelected(current.SelectedLenses, lens) {
		return true, 1, fmt.Errorf("review capture-result: lens %q is not mandated for this review", lens)
	}
	if _, exists := current.LensResults[lens]; exists {
		return true, 1, fmt.Errorf("review capture-result: lens %q already captured", lens)
	}

	identity, _, err := buildCandidate(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review capture-result: building candidate identity: %w", err)
	}
	if computeSubjectHash(identity) != current.SubjectHash {
		return true, 1, fmt.Errorf("review capture-result: workspace no longer matches the frozen review subject; run `review start` again")
	}

	data, err := readFile(filePath)
	if err != nil {
		return true, 1, fmt.Errorf("review capture-result: reading result file: %w", err)
	}
	if !json.Valid(data) {
		return true, 1, fmt.Errorf("review capture-result: result file %q is not valid JSON", filePath)
	}

	result := reviewstore.LensResult{
		LensID:      lens,
		SubjectHash: current.SubjectHash,
		Findings:    json.RawMessage(data),
		CapturedAt:  reviewNow().UTC().Format(time.RFC3339),
	}
	if current.LensResults == nil {
		current.LensResults = make(map[rdd.Lens]reviewstore.LensResult)
	}
	current.LensResults[lens] = result

	if allMandatedCaptured(current.SelectedLenses, current.LensResults) {
		if err := rdd.ValidateTransition(current.State, rdd.StateFindingsFrozen); err != nil {
			return true, 1, fmt.Errorf("review capture-result: %w", err)
		}
		current.State = rdd.StateFindingsFrozen
	}
	current.UpdatedAt = reviewNow().UTC().Format(time.RFC3339)

	if err := store.SaveState(current); err != nil {
		return true, 1, fmt.Errorf("review capture-result: saving review state: %w", err)
	}

	fmt.Fprintf(out, "captured lens %s (state=%s)\n", lens, current.State)
	return true, 0, nil
}

// finalizeRequiresFix decides, once frozen findings have been evaluated
// (state evidence_classified), whether the candidate needs a fix before
// final verification. R-2 has no automated evidence-classification logic —
// that determination requires external input (e.g. a human evaluating lens
// findings) `review finalize` does not yet have access to — so the default
// always proceeds to ready_final_verification (design "review finalize"
// heuristic: "always route to ready_final_verification ... since fix
// determination requires external input not yet available"). Package-level
// var seam (design pattern established by buildCandidate/readFile) so tests
// can exercise the fix_required path without real evidence-classification
// logic; a real implementation is an extension point for R-3.
var finalizeRequiresFix = func(*reviewstore.ReviewState) bool {
	return false
}

// finalizeVerificationOutcome decides the terminal outcome of final
// verification (state final_verifying): approved by default, unless a lens
// result signals failure. R-2 has no automated lens-result failure detection
// yet (extension point for R-3), so the default always approves.
// Package-level var seam so tests can drive the escalated outcome.
var finalizeVerificationOutcome = func(*reviewstore.ReviewState) rdd.LifecycleState {
	return rdd.StateApproved
}

// advanceState validates the transition from current.State to next via
// rdd.ValidateTransition, then persists it via SaveState. On success it
// mutates current.State/UpdatedAt in place; on failure current.State is left
// unchanged, so a failed multi-step finalize never leaves the in-memory or
// on-disk state at an intermediate, half-applied value (design "review
// finalize is multi-step: ... If ANY intermediate transition fails, the
// state stays at the last valid state (no partial corruption)").
func advanceState(store *reviewstore.Store, current *reviewstore.ReviewState, next rdd.LifecycleState) error {
	if err := rdd.ValidateTransition(current.State, next); err != nil {
		return err
	}
	prev := current.State
	current.State = next
	current.UpdatedAt = reviewNow().UTC().Format(time.RFC3339)
	if err := store.SaveState(current); err != nil {
		current.State = prev
		return err
	}
	return nil
}

// reviewFinalize handles `review finalize` (spec review-cli-commands;
// design "Data Flow"): loads the current review state, requires it to be
// findings_frozen, then drives a multi-step atomic transition through
// evidence_classified -> (ready_final_verification -> final_verifying ->
// approved|escalated) or, when finalizeRequiresFix says a fix is needed,
// evidence_classified -> fix_required -> fixing -> fix_validating ->
// escalated (no fix CLI exists in R-2, so finalize itself walks the fix
// states straight to escalated rather than invoking any unbuilt fix verb —
// design notes "No fix CLI in R-2"). Each step is validated individually via
// advanceState, so a failure partway through leaves the state at its last
// successfully committed value. Reaching a terminal state issues the review's
// one and only ReviewReceipt.
func reviewFinalize(args []string, out io.Writer) (bool, int, error) {
	workspace, err := resolveWorkspace()
	if err != nil {
		return true, 1, fmt.Errorf("review finalize: resolving workspace: %w", err)
	}

	commonDir, err := reviewstore.GitCommonDir(workspace)
	if err != nil {
		return true, 1, fmt.Errorf("review finalize: resolving git common dir: %w", err)
	}

	store := reviewstore.NewStore(reviewAuthorityDir(commonDir))
	current, err := store.LoadState()
	if err != nil {
		return true, 1, fmt.Errorf("review finalize: loading review state: %w", err)
	}
	if current == nil {
		return true, 1, fmt.Errorf("review finalize: no review state found; run `review start` first")
	}
	if current.State != rdd.StateFindingsFrozen {
		return true, 1, fmt.Errorf(
			"review finalize: review state is %q, want %q (run `review capture-result` for all mandated lenses first)",
			current.State, rdd.StateFindingsFrozen,
		)
	}

	if err := advanceState(store, current, rdd.StateEvidenceClassified); err != nil {
		return true, 1, fmt.Errorf("review finalize: %w", err)
	}

	if finalizeRequiresFix(current) {
		fixPath := []rdd.LifecycleState{
			rdd.StateFixRequired,
			rdd.StateFixing,
			rdd.StateFixValidating,
			rdd.StateEscalated,
		}
		for _, next := range fixPath {
			if err := advanceState(store, current, next); err != nil {
				return true, 1, fmt.Errorf("review finalize: %w", err)
			}
		}
	} else {
		if err := advanceState(store, current, rdd.StateReadyFinalVerification); err != nil {
			return true, 1, fmt.Errorf("review finalize: %w", err)
		}
		if err := advanceState(store, current, rdd.StateFinalVerifying); err != nil {
			return true, 1, fmt.Errorf("review finalize: %w", err)
		}
		outcome := finalizeVerificationOutcome(current)
		if err := advanceState(store, current, outcome); err != nil {
			return true, 1, fmt.Errorf("review finalize: %w", err)
		}
	}

	if rdd.IsTerminal(current.State) {
		receipt := &reviewstore.ReviewReceipt{
			SchemaVersion: 1,
			Candidate:     current.Candidate,
			Tier:          current.Tier,
			Outcome:       string(current.State),
			IssuedAt:      reviewNow().UTC().Format(time.RFC3339),
		}
		if err := store.WriteReceipt(receipt); err != nil {
			return true, 1, fmt.Errorf("review finalize: writing receipt: %w", err)
		}
	}

	fmt.Fprintf(out, "review finalized: state=%s\n", current.State)
	return true, 0, nil
}

// reviewSchema dispatches `review schema <verb>`.
func reviewSchema(args []string, out io.Writer) (bool, int, error) {
	if len(args) == 0 {
		fmt.Fprintln(out, reviewUsage)
		return true, 1, fmt.Errorf("review schema: verb required (reviewer)")
	}

	verb := args[0]
	switch verb {
	case "reviewer":
		return reviewSchemaReviewer(out)
	default:
		fmt.Fprintf(out, "review: unknown schema verb %q\n", verb)
		fmt.Fprintln(out, reviewUsage)
		return true, 1, nil
	}
}

// reviewFindingsSchema is the fixed JSON schema describing a valid
// LensResult.Findings submission for `review capture-result` (spec
// review-cli-commands: "review schema reviewer outputs the JSON schema a
// reviewer must produce"). It documents the findings envelope a reviewer
// writes to a result file, not reviewstore.LensResult itself — LensResult
// wraps Findings with bookkeeping fields (lens ID, subject hash, captured
// timestamp) the reviewer never supplies.
const reviewFindingsSchema = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "LensResult.Findings",
  "description": "Expected JSON shape for a reviewer's findings submission, as passed to \"review capture-result --lens <id> <file>\".",
  "type": "object",
  "properties": {
    "issues": {
      "type": "array",
      "description": "Findings raised by this lens. An empty array means no issues found.",
      "items": {
        "type": "object",
        "properties": {
          "severity": {
            "type": "string",
            "enum": ["info", "warning", "critical"],
            "description": "Severity of the finding."
          },
          "path": {
            "type": "string",
            "description": "File path the finding applies to."
          },
          "message": {
            "type": "string",
            "description": "Human-readable description of the finding."
          }
        },
        "required": ["severity", "message"]
      }
    }
  },
  "required": ["issues"]
}
`

// reviewSchemaReviewer handles `review schema reviewer` (spec
// review-cli-commands; design "CLI -> State Mapping"): prints the fixed JSON
// schema for a reviewer's findings submission. It is read-only and works in
// any review state — it performs no LoadState/SaveState call at all.
func reviewSchemaReviewer(out io.Writer) (bool, int, error) {
	fmt.Fprintln(out, strings.TrimSpace(reviewFindingsSchema))
	return true, 0, nil
}
