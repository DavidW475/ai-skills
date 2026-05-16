package lockfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DavidW475/ai-skills/internal/config"
)

func makeEntry(name string) Entry {
	return Entry{
		Name:      name,
		Resolved:  "registry.example.com/ns/" + name + ":v1.0.0",
		Digest:    "sha256:abc123",
		Installed: "/home/user/.agent/skills/" + name,
	}
}

// ---- Upsert ----

func TestUpsert_add(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	if len(lf.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Name != "ansible" {
		t.Errorf("expected name=ansible, got %q", lf.Skills[0].Name)
	}
}

func TestUpsert_update(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	updated := makeEntry("ansible")
	updated.Digest = "sha256:newdigest"
	lf.Upsert(updated)

	if len(lf.Skills) != 1 {
		t.Fatalf("Upsert should not add duplicate, got %d entries", len(lf.Skills))
	}
	if lf.Skills[0].Digest != "sha256:newdigest" {
		t.Errorf("Upsert did not update digest: got %q", lf.Skills[0].Digest)
	}
}

func TestUpsert_multiple(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))
	lf.Upsert(makeEntry("terraform"))
	lf.Upsert(makeEntry("kubectl"))

	if len(lf.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(lf.Skills))
	}
}

// ---- Find ----

func TestFind_existing(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))
	lf.Upsert(makeEntry("terraform"))

	e := lf.Find("terraform")
	if e == nil {
		t.Fatal("Find returned nil for existing entry")
	}
	if e.Name != "terraform" {
		t.Errorf("Find returned wrong entry: %q", e.Name)
	}
}

func TestFind_notFound(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	if e := lf.Find("nonexistent"); e != nil {
		t.Errorf("Find should return nil for missing entry, got %+v", e)
	}
}

func TestFind_empty(t *testing.T) {
	lf := &LockFile{}
	if e := lf.Find("anything"); e != nil {
		t.Errorf("Find on empty lockfile should return nil, got %+v", e)
	}
}

// ---- Remove ----

func TestRemove_existing(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))
	lf.Upsert(makeEntry("terraform"))

	ok := lf.Remove("ansible")
	if !ok {
		t.Error("Remove returned false for existing entry")
	}
	if len(lf.Skills) != 1 {
		t.Errorf("expected 1 skill after remove, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Name != "terraform" {
		t.Errorf("wrong skill remaining: %q", lf.Skills[0].Name)
	}
}

func TestRemove_notFound(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	ok := lf.Remove("nonexistent")
	if ok {
		t.Error("Remove should return false for nonexistent entry")
	}
	if len(lf.Skills) != 1 {
		t.Errorf("Remove should not change length, got %d", len(lf.Skills))
	}
}

func TestRemove_last(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	ok := lf.Remove("ansible")
	if !ok {
		t.Error("Remove returned false")
	}
	if len(lf.Skills) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(lf.Skills))
	}
}

// ---- Find returns pointer into slice (mutation test) ----

func TestFind_pointerMutation(t *testing.T) {
	lf := &LockFile{}
	lf.Upsert(makeEntry("ansible"))

	e := lf.Find("ansible")
	e.Digest = "sha256:mutated"

	if lf.Skills[0].Digest != "sha256:mutated" {
		t.Error("Find should return a pointer to the actual slice element")
	}
}

// ---- Load ----

func TestLoad_missing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	lf, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(lf.Skills) != 0 {
		t.Errorf("expected empty lockfile, got %d skills", len(lf.Skills))
	}
}

func TestLoad_invalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, config.DirName)
	os.MkdirAll(dir, 0o755)                                                                    //nolint:errcheck
	os.WriteFile(filepath.Join(dir, config.InstalledFile), []byte("skills: [unclosed"), 0o644) //nolint:errcheck

	if _, err := Load(); err == nil {
		t.Error("Load() should error on invalid YAML")
	}
}

// ---- Save / Load round-trip ----

func TestSave_roundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	lf := &LockFile{}
	lf.Upsert(Entry{Name: "ansible", Resolved: "reg/ns/ansible:v1.0.0", Digest: "sha256:abc", Installed: "/i/ansible"})
	lf.Upsert(Entry{Name: "terraform", Resolved: "reg/ns/terraform:v2.0.0", Digest: "sha256:def", Installed: "/i/terraform"})

	if err := Save(lf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if len(loaded.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(loaded.Skills))
	}
	if loaded.Skills[0].Name != "ansible" {
		t.Errorf("Skills[0].Name = %q, want ansible", loaded.Skills[0].Name)
	}
	if loaded.Skills[1].Digest != "sha256:def" {
		t.Errorf("Skills[1].Digest = %q, want sha256:def", loaded.Skills[1].Digest)
	}
}

func TestSave_createsDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Directory does not exist yet
	lf := &LockFile{}
	if err := Save(lf); err != nil {
		t.Fatalf("Save() should create parent directory: %v", err)
	}

	path := filepath.Join(home, config.DirName, config.InstalledFile)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s after Save: %v", path, err)
	}
}
