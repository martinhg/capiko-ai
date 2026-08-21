package rdd

// ReviewMode is the resolved kill-switch mode: whether review enforcement is
// active. R-1 supports only two states — there is no "unset" value; the
// absence of a valid ModeRecord at a scope is resolved by ResolveMode, never
// represented as a ReviewMode value itself.
type ReviewMode string

const (
	// ModeManaged means review enforcement is active. It is the fail-closed
	// default: ResolveMode returns it whenever no scope carries a valid,
	// explicit ModeDisabled record.
	ModeManaged ReviewMode = "managed"
	// ModeDisabled means review enforcement is explicitly turned off at the
	// scope that set it. Only an explicit, valid record can produce this
	// value — it is never the default.
	ModeDisabled ReviewMode = "disabled"
)

// valid reports whether m is a recognized ReviewMode value. The zero value
// and any unrecognized string are treated as absent/corrupt by ResolveMode.
func (m ReviewMode) valid() bool {
	return m == ModeManaged || m == ModeDisabled
}

// ModeRecord is a persisted kill-switch record at one scope (global or
// clone). SchemaVersion supports forward-compatible evolution (design
// "All JSON files carry schema_version: 1").
type ModeRecord struct {
	SchemaVersion int        `json:"schema_version"`
	Mode          ReviewMode `json:"mode"`
	UpdatedAt     string     `json:"updated_at"`
}

// ResolveMode resolves the effective ReviewMode from an optional global-scope
// and an optional clone-scope ModeRecord (spec rdd-kill-switch; design
// "Kill Switch Scope Resolution"). Clone overrides global when both are
// present and valid — the most specific scope wins. A nil record, or a
// record whose Mode is empty or unrecognized (corrupt), is treated as absent
// at that scope. If neither scope yields a valid mode, ResolveMode fails
// closed to ModeManaged: enforcement is never silently disabled by a
// missing, unreadable, or corrupt record (security kernel invariant "Kill
// switch"). It is a pure function: no I/O.
func ResolveMode(global, clone *ModeRecord) ReviewMode {
	if clone != nil && clone.Mode.valid() {
		return clone.Mode
	}
	if global != nil && global.Mode.valid() {
		return global.Mode
	}
	return ModeManaged
}
