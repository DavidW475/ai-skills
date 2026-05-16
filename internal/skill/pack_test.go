package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// makeTestSkillDir creates a minimal valid skill directory in a temp dir.
func makeTestSkillDir(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	m := &Manifest{Name: name, Version: "1.0.0", Description: "A test skill."}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, SkillFile), []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
	return dir
}

// ---- Pack ----

func TestPack_missingManifest(t *testing.T) {
	dir := t.TempDir()
	if err := Pack(dir, &bytes.Buffer{}); err == nil {
		t.Error("Pack() should fail when skill.yaml is missing")
	}
}

func TestPack_missingSkillMD(t *testing.T) {
	dir := t.TempDir()
	m := &Manifest{Name: "test-skill", Version: "1.0.0"}
	if err := WriteManifest(dir, m); err != nil {
		t.Fatal(err)
	}
	if err := Pack(dir, &bytes.Buffer{}); err == nil {
		t.Error("Pack() should fail when SKILL.md is missing")
	}
}

func TestPack_producesNonEmptyOutput(t *testing.T) {
	dir := makeTestSkillDir(t, "test-skill")

	var buf bytes.Buffer
	if err := Pack(dir, &buf); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("Pack() produced empty output")
	}
}

// ---- Unpack ----

func TestPack_Unpack_roundTrip(t *testing.T) {
	src := makeTestSkillDir(t, "roundtrip")

	// Add a subdirectory with a file to exercise directory entries.
	subDir := filepath.Join(src, "assets")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "data.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Pack(src, &buf); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	dst := t.TempDir()
	if err := Unpack(&buf, dst); err != nil {
		t.Fatalf("Unpack() error: %v", err)
	}

	for _, rel := range []string{ManifestFile, SkillFile, filepath.Join("assets", "data.txt")} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected %s in unpacked dir: %v", rel, err)
		}
	}
}

func TestUnpack_invalidGzip(t *testing.T) {
	r := bytes.NewReader([]byte("this is not gzip data"))
	if err := Unpack(r, t.TempDir()); err == nil {
		t.Error("Unpack() should fail on invalid gzip stream")
	}
}

func TestUnpack_pathTraversal(t *testing.T) {
	// Build a crafted archive that contains a path traversal entry.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "../escape.txt",
		Size:     4,
		Mode:     0o644,
	}
	tw.WriteHeader(hdr) //nolint:errcheck
	tw.Write([]byte("evil"))
	tw.Close() //nolint:errcheck
	gz.Close() //nolint:errcheck

	if err := Unpack(&buf, t.TempDir()); err == nil {
		t.Error("Unpack() should reject path traversal entries")
	}
}

func TestUnpack_absolutePath(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "/etc/evil.txt",
		Size:     4,
		Mode:     0o644,
	}
	tw.WriteHeader(hdr) //nolint:errcheck
	tw.Write([]byte("evil"))
	tw.Close() //nolint:errcheck
	gz.Close() //nolint:errcheck

	if err := Unpack(&buf, t.TempDir()); err == nil {
		t.Error("Unpack() should reject absolute paths in archive")
	}
}

func TestUnpack_dirEntry(t *testing.T) {
	// Archive that only contains a directory entry — should unpack cleanly.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "mydir/", Mode: 0o755}) //nolint:errcheck
	tw.Close()                                                                      //nolint:errcheck
	gz.Close()                                                                      //nolint:errcheck

	dst := t.TempDir()
	if err := Unpack(&buf, dst); err != nil {
		t.Fatalf("Unpack() error on dir entry: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "mydir")); err != nil {
		t.Errorf("expected mydir to be created: %v", err)
	}
}
