package skill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestNameFromArchive_valid(t *testing.T) {
	dir := makeTestSkillDir(t, "archive-skill")

	var buf bytes.Buffer
	if err := Pack(dir, &buf); err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	name, err := NameFromArchive(&buf)
	if err != nil {
		t.Fatalf("NameFromArchive() error: %v", err)
	}
	if name != "archive-skill" {
		t.Errorf("NameFromArchive() = %q, want archive-skill", name)
	}
}

func TestNameFromArchive_invalidGzip(t *testing.T) {
	r := bytes.NewReader([]byte("not gzip"))
	if _, err := NameFromArchive(r); err == nil {
		t.Error("NameFromArchive() should fail on invalid gzip")
	}
}

func TestNameFromArchive_noManifest(t *testing.T) {
	// A valid gzip/tar archive that does NOT contain skill.yaml.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("hello world")
	tw.WriteHeader(&tar.Header{Name: "README.md", Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0o644}) //nolint:errcheck
	tw.Write(body)                                                                                              //nolint:errcheck
	tw.Close()                                                                                                  //nolint:errcheck
	gz.Close()                                                                                                  //nolint:errcheck

	if _, err := NameFromArchive(&buf); err == nil {
		t.Error("NameFromArchive() should fail when skill.yaml is absent")
	}
}

func TestNameFromArchive_invalidManifestYAML(t *testing.T) {
	// Archive with a skill.yaml that is invalid YAML.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("name: [unclosed bracket")
	tw.WriteHeader(&tar.Header{Name: ManifestFile, Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0o644}) //nolint:errcheck
	tw.Write(body)                                                                                               //nolint:errcheck
	tw.Close()                                                                                                   //nolint:errcheck
	gz.Close()                                                                                                   //nolint:errcheck

	if _, err := NameFromArchive(&buf); err == nil {
		t.Error("NameFromArchive() should fail on invalid manifest YAML")
	}
}

func TestNameFromArchive_invalidManifestFields(t *testing.T) {
	// Archive with a skill.yaml that parses but fails Validate (no version).
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("name: my-skill\n") // missing version
	tw.WriteHeader(&tar.Header{Name: ManifestFile, Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0o644}) //nolint:errcheck
	tw.Write(body)                                                                                               //nolint:errcheck
	tw.Close()                                                                                                   //nolint:errcheck
	gz.Close()                                                                                                   //nolint:errcheck

	if _, err := NameFromArchive(&buf); err == nil {
		t.Error("NameFromArchive() should fail when manifest validation fails")
	}
}

