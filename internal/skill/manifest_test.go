package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- Manifest.Validate ----

func TestValidate_valid(t *testing.T) {
	m := &Manifest{Name: "my-skill", Version: "1.0.0"}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidate_missingName(t *testing.T) {
	m := &Manifest{Version: "1.0.0"}
	if err := m.Validate(); err == nil {
		t.Error("Validate() should return error for missing name")
	}
}

func TestValidate_missingVersion(t *testing.T) {
	m := &Manifest{Name: "my-skill"}
	if err := m.Validate(); err == nil {
		t.Error("Validate() should return error for missing version")
	}
}

func TestValidate_invalidName(t *testing.T) {
	invalid := []string{"My-Skill", "my skill", "my_skill", "-leading", "trailing-", "UPPERCASE"}
	for _, name := range invalid {
		m := &Manifest{Name: name, Version: "1.0.0"}
		if err := m.Validate(); err == nil {
			t.Errorf("Validate() should return error for name %q", name)
		}
	}
}

func TestValidate_validNames(t *testing.T) {
	valid := []string{"skill", "my-skill", "my-long-skill-name", "skill123", "123skill"}
	for _, name := range valid {
		m := &Manifest{Name: name, Version: "1.0.0"}
		if err := m.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid name %q: %v", name, err)
		}
	}
}

// ---- LoadManifest ----

func writeManifestFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
}

func TestLoadManifest_valid(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, `name: test-skill
version: "1.2.3"
description: A test skill.
author: tester
`)
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error: %v", err)
	}
	if m.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", m.Name, "test-skill")
	}
	if m.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", m.Version, "1.2.3")
	}
	if m.Description != "A test skill." {
		t.Errorf("Description = %q", m.Description)
	}
}

func TestLoadManifest_notFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadManifest(dir); err == nil {
		t.Error("LoadManifest() should error when file does not exist")
	}
}

func TestLoadManifest_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeManifestFile(t, dir, "name: [invalid yaml\n")
	if _, err := LoadManifest(dir); err == nil {
		t.Error("LoadManifest() should error on invalid YAML")
	}
}

func TestLoadManifest_validationError(t *testing.T) {
	dir := t.TempDir()
	// Missing version
	writeManifestFile(t, dir, "name: my-skill\n")
	if _, err := LoadManifest(dir); err == nil {
		t.Error("LoadManifest() should error when manifest fails validation")
	}
}

// ---- WriteManifest ----

func TestWriteManifest_roundTrip(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{
		Name:        "roundtrip",
		Version:     "0.1.0",
		Description: "round trip test",
	}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest() error: %v", err)
	}
	loaded, err := LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() after WriteManifest() error: %v", err)
	}
	if loaded.Name != m.Name || loaded.Version != m.Version || loaded.Description != m.Description {
		t.Errorf("round-trip mismatch: got %+v, want %+v", loaded, m)
	}
}

func TestWriteManifest_invalidManifest(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{} // missing required fields
	if err := WriteManifest(dir, m); err == nil {
		t.Error("WriteManifest() should error for invalid manifest")
	}
}
