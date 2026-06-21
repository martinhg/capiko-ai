// Package backup snapshots Copilot skill directories before capiko mutates
// them, so any change can be undone. Backups live under ~/.capiko/backups/<id>,
// each with a manifest.json and a copy of the affected skills.
package backup

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry records one skill captured in a backup.
type Entry struct {
	Skill   string `json:"skill"`   // skill directory name
	Existed bool   `json:"existed"` // whether it existed before the mutation
}

// FileEntry records one standalone file captured in a backup (e.g. Copilot's
// global instructions file when a persona is applied).
type FileEntry struct {
	Label   string `json:"label"`   // base name used for the snapshot copy
	Path    string `json:"path"`    // original absolute path
	Existed bool   `json:"existed"` // whether it existed before the mutation
}

// Manifest is the metadata stored alongside a backup's files.
type Manifest struct {
	ID        string      `json:"id"` // timestamp id, also the backup dir name
	CreatedAt time.Time   `json:"created_at"`
	Version   string      `json:"version"` // capiko version that created it
	Reason    string      `json:"reason"`  // install | uninstall | sync | persona
	Entries   []Entry     `json:"entries"`
	Files     []FileEntry `json:"files,omitempty"`
}

// Store manages backups under a root directory (default ~/.capiko/backups).
type Store struct{ dir string }

// NewStore returns a store rooted at dir.
func NewStore(dir string) *Store { return &Store{dir: dir} }

// DefaultStore returns a store at ~/.capiko/backups.
func DefaultStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Store{dir: filepath.Join(home, ".capiko", "backups")}, nil
}

// Create snapshots the given skills from skillsDir into a new backup and returns
// its id. Skills that do not currently exist are recorded (Existed=false) so a
// restore can remove them, returning to the pre-mutation state.
func (s *Store) Create(skillsDir, reason, version string, skills []string) (string, error) {
	id, dir := s.newBackupDir()
	entries, err := snapshotSkills(skillsDir, dir, skills)
	if err != nil {
		return "", err
	}
	man := Manifest{
		ID:        id,
		CreatedAt: time.Now().UTC(),
		Version:   version,
		Reason:    reason,
		Entries:   entries,
	}
	if err := writeManifest(dir, man); err != nil {
		return "", err
	}
	return id, nil
}

// CreateWithAgents snapshots skills (directory layout) and agent files (the flat
// <name>.agent.md layout under agentsDir) into a single backup, so a destructive
// op that mutates both can be undone atomically. Agents are recorded as
// FileEntry alongside the skill Entries in one manifest; Restore reinstates
// both. Used by InstallAll/UninstallAll, which touch skills and agents together.
func (s *Store) CreateWithAgents(skillsDir, agentsDir, reason, version string, skills, agents []string) (string, error) {
	id, dir := s.newBackupDir()
	entries, err := snapshotSkills(skillsDir, dir, skills)
	if err != nil {
		return "", err
	}
	paths := make([]string, len(agents))
	for i, name := range agents {
		paths[i] = filepath.Join(agentsDir, name+".agent.md")
	}
	files, err := snapshotFiles(dir, paths)
	if err != nil {
		return "", err
	}
	man := Manifest{
		ID:        id,
		CreatedAt: time.Now().UTC(),
		Version:   version,
		Reason:    reason,
		Entries:   entries,
		Files:     files,
	}
	if err := writeManifest(dir, man); err != nil {
		return "", err
	}
	return id, nil
}

// CreateFiles snapshots the given absolute file paths into a new backup and
// returns its id. Files that do not currently exist are recorded (Existed=false)
// so a restore can remove them, returning to the pre-mutation state. Used for
// standalone files outside the skills tree (e.g. copilot-instructions.md).
func (s *Store) CreateFiles(reason, version string, paths []string) (string, error) {
	id, dir := s.newBackupDir()
	files, err := snapshotFiles(dir, paths)
	if err != nil {
		return "", err
	}

	man := Manifest{
		ID:        id,
		CreatedAt: time.Now().UTC(),
		Version:   version,
		Reason:    reason,
		Files:     files,
	}
	if err := writeManifest(dir, man); err != nil {
		return "", err
	}
	return id, nil
}

// List returns all backups, newest first. A missing backups dir is not an error.
func (s *Store) List() ([]Manifest, error) {
	dirs, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Manifest
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		man, err := readManifest(filepath.Join(s.dir, d.Name()))
		if err != nil {
			continue // skip anything without a valid manifest
		}
		out = append(out, man)
	}
	// IDs are sortable timestamps; descending gives newest-first and is stable
	// even when two backups share a CreatedAt instant.
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

// Restore returns skillsDir to the state captured by backup id: each recorded
// skill is removed and, if it existed, its snapshot is copied back.
func (s *Store) Restore(skillsDir, id string) error {
	man, err := readManifest(filepath.Join(s.dir, id))
	if err != nil {
		return err
	}
	for _, e := range man.Entries {
		dst := filepath.Join(skillsDir, e.Skill)
		if err := os.RemoveAll(dst); err != nil {
			return err
		}
		if e.Existed {
			if err := copyDir(filepath.Join(s.dir, id, "skills", e.Skill), dst); err != nil {
				return err
			}
		}
	}
	for _, f := range man.Files {
		if f.Existed {
			if err := copyFile(filepath.Join(s.dir, id, "files", f.Label), f.Path); err != nil {
				return err
			}
		} else if err := os.Remove(f.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// Delete removes a backup. It refuses anything without a manifest.json so a bad
// id can never delete arbitrary files.
func (s *Store) Delete(id string) error {
	dir := filepath.Join(s.dir, id)
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// --- helpers ---

// newBackupDir returns a fresh unique backup id and its absolute directory.
// Nanosecond precision plus a collision guard guarantees a unique id even for
// backups created within the same instant.
func (s *Store) newBackupDir() (id, dir string) {
	base := time.Now().UTC().Format("20060102T150405.000000000")
	id, dir = base, filepath.Join(s.dir, base)
	for i := 1; isDir(dir); i++ {
		id = fmt.Sprintf("%s-%d", base, i)
		dir = filepath.Join(s.dir, id)
	}
	return id, dir
}

// snapshotSkills copies each named skill directory from skillsDir into dir/skills
// and records an Entry per skill. Skills that do not currently exist are recorded
// (Existed=false) so a restore can remove them, returning to the pre-mutation state.
func snapshotSkills(skillsDir, dir string, skills []string) ([]Entry, error) {
	var entries []Entry
	for _, name := range skills {
		src := filepath.Join(skillsDir, name)
		existed := isDir(src)
		if existed {
			if err := copyDir(src, filepath.Join(dir, "skills", name)); err != nil {
				return nil, err
			}
		}
		entries = append(entries, Entry{Skill: name, Existed: existed})
	}
	return entries, nil
}

// snapshotFiles copies each path into dir/files and records a FileEntry per path.
// Files that do not currently exist are recorded (Existed=false) so a restore can
// remove them, returning to the pre-mutation state.
func snapshotFiles(dir string, paths []string) ([]FileEntry, error) {
	var files []FileEntry
	for _, p := range paths {
		existed := isFile(p)
		label := filepath.Base(p)
		if existed {
			if err := copyFile(p, filepath.Join(dir, "files", label)); err != nil {
				return nil, err
			}
		}
		files = append(files, FileEntry{Label: label, Path: p, Existed: existed})
	}
	return files, nil
}

func writeManifest(dir string, man Manifest) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
}

func readManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Manifest{}, err
	}
	var man Manifest
	if err := json.Unmarshal(data, &man); err != nil {
		return Manifest{}, err
	}
	return man, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
