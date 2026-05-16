package lockfile

import (
	"testing"
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
