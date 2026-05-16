package sources

import (
	"testing"
)

// ---- Add ----

func TestAdd_new(t *testing.T) {
	f := &File{}
	ok := f.Add("registry.example.com/ns")
	if !ok {
		t.Error("Add should return true for a new source")
	}
	if len(f.Sources) != 1 || f.Sources[0] != "registry.example.com/ns" {
		t.Errorf("unexpected Sources: %v", f.Sources)
	}
}

func TestAdd_duplicate(t *testing.T) {
	f := &File{Sources: []string{"registry.example.com/ns"}}
	ok := f.Add("registry.example.com/ns")
	if ok {
		t.Error("Add should return false for a duplicate source")
	}
	if len(f.Sources) != 1 {
		t.Errorf("Sources should still have 1 entry, got %d", len(f.Sources))
	}
}

func TestAdd_multiple(t *testing.T) {
	f := &File{}
	f.Add("registry.example.com/a")
	f.Add("registry.example.com/b")
	f.Add("registry.example.com/c")

	if len(f.Sources) != 3 {
		t.Errorf("expected 3 sources, got %d", len(f.Sources))
	}
}

// ---- Remove ----

func TestRemove_existing(t *testing.T) {
	f := &File{Sources: []string{"reg-a", "reg-b", "reg-c"}}
	ok := f.Remove("reg-b")
	if !ok {
		t.Error("Remove should return true for existing source")
	}
	if len(f.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(f.Sources))
	}
	for _, s := range f.Sources {
		if s == "reg-b" {
			t.Error("removed source still present")
		}
	}
}

func TestRemove_notFound(t *testing.T) {
	f := &File{Sources: []string{"reg-a"}}
	ok := f.Remove("reg-x")
	if ok {
		t.Error("Remove should return false for nonexistent source")
	}
	if len(f.Sources) != 1 {
		t.Errorf("sources should be unchanged, got %d", len(f.Sources))
	}
}

func TestRemove_last(t *testing.T) {
	f := &File{Sources: []string{"only"}}
	ok := f.Remove("only")
	if !ok {
		t.Error("Remove should return true")
	}
	if len(f.Sources) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(f.Sources))
	}
}

func TestRemove_first(t *testing.T) {
	f := &File{Sources: []string{"first", "second"}}
	f.Remove("first")
	if len(f.Sources) != 1 || f.Sources[0] != "second" {
		t.Errorf("unexpected Sources after removing first: %v", f.Sources)
	}
}

// ---- Add + Remove round-trip ----

func TestAddRemoveRoundTrip(t *testing.T) {
	f := &File{}
	f.Add("a")
	f.Add("b")
	f.Add("a") // duplicate — no-op
	f.Remove("a")

	if len(f.Sources) != 1 || f.Sources[0] != "b" {
		t.Errorf("unexpected Sources after round-trip: %v", f.Sources)
	}
}
