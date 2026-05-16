package lockfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DavidW475/ai-skills/internal/config"
)

// Entry records an installed skill with its resolved ref, digest, and local path.
type Entry struct {
	Name      string `yaml:"name"      json:"name"`
	Resolved  string `yaml:"resolved"  json:"resolved"` // full OCI ref with tag
	Digest    string `yaml:"digest"    json:"digest"`
	Installed string `yaml:"installed" json:"installed"` // absolute path to the installed skill directory
}

// LockFile is the on-disk representation of ~/.ai-skills/installed.yaml.
type LockFile struct {
	Skills []Entry `yaml:"skills"`
}

// Load reads ~/.ai-skills/installed.yaml. Returns an empty LockFile if it does not exist.
func Load() (*LockFile, error) {
	path, err := installedPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &LockFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lockfile: read %s: %w", path, err)
	}
	var lf LockFile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("lockfile: parse %s: %w", path, err)
	}
	return &lf, nil
}

// Save writes the installed index to ~/.ai-skills/installed.yaml, creating the directory if needed.
func Save(lf *LockFile) error {
	dir, err := config.UserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("lockfile: mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("lockfile: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, config.InstalledFile), data, 0o644)
}

func installedPath() (string, error) {
	dir, err := config.UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, config.InstalledFile), nil
}

// Upsert adds or updates the entry for the given skill name.
func (lf *LockFile) Upsert(e Entry) {
	for i, existing := range lf.Skills {
		if existing.Name == e.Name {
			lf.Skills[i] = e
			return
		}
	}
	lf.Skills = append(lf.Skills, e)
}

// Find returns the entry for the given skill name, or nil if not found.
func (lf *LockFile) Find(name string) *Entry {
	for i := range lf.Skills {
		if lf.Skills[i].Name == name {
			return &lf.Skills[i]
		}
	}
	return nil
}

// Remove deletes the entry for the given skill name.
// Returns false if no such entry existed.
func (lf *LockFile) Remove(name string) bool {
	for i, e := range lf.Skills {
		if e.Name == name {
			lf.Skills = append(lf.Skills[:i], lf.Skills[i+1:]...)
			return true
		}
	}
	return false
}
