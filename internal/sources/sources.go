package sources

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/DavidW475/ai-skills/internal/config"
)

// File is the on-disk representation of ~/.ai-skills/sources.
type File struct {
	Sources []string `yaml:"sources"`
}

// Load reads ~/.ai-skills/sources. Returns an empty File if it does not exist.
func Load() (*File, error) {
	path, err := sourcesPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &File{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("sources: read %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("sources: parse %s: %w", path, err)
	}
	return &f, nil
}

// Save writes the sources file to ~/.ai-skills/sources, creating the directory if needed.
func Save(f *File) error {
	dir, err := config.UserDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("sources: mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("sources: marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, config.SourcesFile), data, 0o644)
}

func sourcesPath() (string, error) {
	dir, err := config.UserDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, config.SourcesFile), nil
}

// Add appends source if not already present. Returns true if added.
func (f *File) Add(source string) bool {
	for _, s := range f.Sources {
		if s == source {
			return false
		}
	}
	f.Sources = append(f.Sources, source)
	return true
}

// Remove removes source. Returns true if removed.
func (f *File) Remove(source string) bool {
	for i, s := range f.Sources {
		if s == source {
			f.Sources = append(f.Sources[:i], f.Sources[i+1:]...)
			return true
		}
	}
	return false
}
