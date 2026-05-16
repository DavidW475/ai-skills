package skill

import (
	"bytes"
	_ "embed" // required to register the go:embed directive used below
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"text/template"

	"gopkg.in/yaml.v3"
)

const ManifestFile = "skill.yaml"
const SkillFile = "SKILL.md"

//go:embed templates/SKILL.md.tmpl
var skillMDTemplate string

var validName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// Manifest is the content of a skill.yaml file.
type Manifest struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description,omitempty"`
	Author      string            `yaml:"author,omitempty"`
	Tags        []string          `yaml:"tags,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Validate checks that all required fields are present and well-formed.
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return errors.New("manifest: name is required")
	}
	if !validName.MatchString(m.Name) {
		return fmt.Errorf("manifest: name %q must be lowercase alphanumeric with hyphens", m.Name)
	}
	if m.Version == "" {
		return errors.New("manifest: version is required")
	}
	return nil
}

// LoadManifest reads and parses the skill.yaml file inside dir.
func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, ManifestFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: cannot read %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: cannot parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// WriteManifest writes a Manifest to dir/skill.yaml.
func WriteManifest(dir string, m *Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("manifest: cannot marshal: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, ManifestFile), data, 0o644)
}

// Scaffold creates a minimal skill directory at dir.
func Scaffold(dir, name, version string) error {
	m := &Manifest{Name: name, Version: version, Description: "A new AI skill."}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := WriteManifest(dir, m); err != nil {
		return err
	}
	skillMD := filepath.Join(dir, SkillFile)
	if _, err := os.Stat(skillMD); errors.Is(err, os.ErrNotExist) {
		tmpl, err := template.New("skill").Parse(skillMDTemplate)
		if err != nil {
			return fmt.Errorf("skill template: %w", err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, m); err != nil {
			return fmt.Errorf("skill template execute: %w", err)
		}
		return os.WriteFile(skillMD, buf.Bytes(), 0o644)
	}
	return nil
}
