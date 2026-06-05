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
	Persona   string                 `json:"persona,omitempty"` // active persona id, "" = unmanaged
}

// SkillRecord is what capiko knows about one skill it installed.
type SkillRecord struct {
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
			return &State{Skills: map[string]SkillRecord{}}, nil
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
