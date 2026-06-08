// Package state persists what capiko manages, in ~/.capiko/state.json.
//
// It is the substrate other foundations build on: backups, upgrade detection,
// and the update banner all read from here. Storing a content checksum per
// skill lets capiko detect drift (catalog changed, disk stale) later.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// State is capiko's persisted view of its managed installation.
type State struct {
	Version   string                 `json:"version"`
	UpdatedAt time.Time              `json:"updated_at"`
	Skills    map[string]SkillRecord `json:"skills"`
	Agents    map[string]AgentRecord `json:"agents,omitempty"`
	Persona   string                 `json:"persona,omitempty"`    // active persona id, "" = unmanaged
	SDDModels map[string]string      `json:"sdd_models,omitempty"` // SDD phase → model, empty = SDD unmanaged
	StrictTDD bool                   `json:"strict_tdd,omitempty"` // SDD apply/verify must follow strict TDD
	// InstructionsInstalled is true once the user installs the curated scoped
	// instruction files; sync re-applies them only when managed, mirroring persona/SDD.
	InstructionsInstalled bool `json:"instructions_installed,omitempty"`
	// Engram records the managed engram backend configuration; nil = unmanaged.
	// Sync re-applies the engram MCP wiring only when it is set, mirroring persona/SDD.
	Engram *EngramRecord `json:"engram,omitempty"`
}

// EngramRecord is the managed engram backend configuration. It never stores the
// cloud token (a secret) — only the non-secret server URL; the token is resolved
// from the environment at runtime via the ${ENGRAM_CLOUD_TOKEN} reference.
type EngramRecord struct {
	Enabled      bool     `json:"enabled"`
	ArtifactMode string   `json:"artifact_mode,omitempty"` // engram|openspec|hybrid|none
	CloudServer  string   `json:"cloud_server,omitempty"`  // server URL only, never the token
	Projects     []string `json:"projects,omitempty"`      // enrolled project names
	Surfaces     []string `json:"surfaces,omitempty"`      // "cli", "vscode"
	VSCodeScope  string   `json:"vscode_scope,omitempty"`  // "workspace" or "user" when the vscode surface is on
	Checksum     string   `json:"checksum,omitempty"`      // of the rendered MCP entry, for drift
}

// SkillRecord is what capiko knows about one skill it installed.
type SkillRecord struct {
	Checksum    string    `json:"checksum"`
	InstalledAt time.Time `json:"installed_at"`
}

// AgentRecord is what capiko knows about one agent it installed.
type AgentRecord struct {
	Checksum    string    `json:"checksum"`
	InstalledAt time.Time `json:"installed_at"`
}

// Installed describes a freshly written skill, passed to Apply.
type Installed struct {
	Name     string
	Checksum string
}

// Checksum returns the hex SHA-256 of a skill's content.
func Checksum(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// Store reads and writes state under a directory (default ~/.capiko).
type Store struct{ dir string }

// NewStore returns a store rooted at dir.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// DefaultStore returns a store at ~/.capiko.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(home, ".capiko")}, nil
}

// Dir is the store's root directory.
func (s *Store) Dir() string { return s.dir }

func (s *Store) path() string { return filepath.Join(s.dir, "state.json") }

// Load reads the state. A missing file yields an empty, ready-to-use state.
func (s *Store) Load() (*State, error) {
	data, err := os.ReadFile(s.path())
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				Skills: map[string]SkillRecord{},
				Agents: map[string]AgentRecord{},
			}, nil
		}
		return nil, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.Skills == nil {
		st.Skills = map[string]SkillRecord{}
	}
	if st.Agents == nil {
		st.Agents = map[string]AgentRecord{}
	}
	return &st, nil
}

// Save writes the state atomically: a temp file is written then renamed, so a
// crash mid-write can never leave a corrupt state.json.
func (s *Store) Save(st *State) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path())
}

// Apply records installed skills and drops removed ones, stamps the version and
// time, and saves.
func (s *Store) Apply(version string, installed []Installed, removed []string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, in := range installed {
		st.Skills[in.Name] = SkillRecord{Checksum: in.Checksum, InstalledAt: now}
	}
	for _, name := range removed {
		delete(st.Skills, name)
	}
	st.Version = version
	st.UpdatedAt = now
	return s.Save(st)
}

// ApplyAgents records installed agents and drops removed ones, stamps the
// version and time, and saves. It mirrors Apply but targets the Agents map.
func (s *Store) ApplyAgents(version string, installed []Installed, removed []string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, in := range installed {
		st.Agents[in.Name] = AgentRecord{Checksum: in.Checksum, InstalledAt: now}
	}
	for _, name := range removed {
		delete(st.Agents, name)
	}
	st.Version = version
	st.UpdatedAt = now
	return s.Save(st)
}

// SetPersona records the active persona id (the empty string means unmanaged).
func (s *Store) SetPersona(id string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	st.Persona = id
	st.UpdatedAt = time.Now().UTC()
	return s.Save(st)
}

// SetSDDModels records the SDD phase→model assignments (nil means unmanaged).
func (s *Store) SetSDDModels(models map[string]string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	st.SDDModels = models
	st.UpdatedAt = time.Now().UTC()
	return s.Save(st)
}

// SetStrictTDD records whether the SDD apply/verify phases must follow strict TDD.
func (s *Store) SetStrictTDD(on bool) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	st.StrictTDD = on
	st.UpdatedAt = time.Now().UTC()
	return s.Save(st)
}

// SetEngram records the managed engram backend configuration (nil means
// unmanaged). It never receives a token: the record carries only the server URL.
func (s *Store) SetEngram(rec *EngramRecord) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	st.Engram = rec
	st.UpdatedAt = time.Now().UTC()
	return s.Save(st)
}

// SetInstructionsInstalled records whether the curated scoped instruction files
// are managed by capiko, so sync re-applies them only after the user installs them.
func (s *Store) SetInstructionsInstalled(on bool) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	st.InstructionsInstalled = on
	st.UpdatedAt = time.Now().UTC()
	return s.Save(st)
}
