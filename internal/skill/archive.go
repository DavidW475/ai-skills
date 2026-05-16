package skill

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// NameFromArchive reads the skill.yaml from a tar.gz stream and returns the
// skill name without fully unpacking the archive.
func NameFromArchive(r io.Reader) (string, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return "", fmt.Errorf("name from archive: not a gzip stream: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("name from archive: %w", err)
		}
		// Accept skill.yaml at the root or one level deep
		if hdr.Name == ManifestFile || hdr.Name == "./"+ManifestFile {
			var m Manifest
			if err := yaml.NewDecoder(io.LimitReader(tr, 64<<10)).Decode(&m); err != nil {
				return "", fmt.Errorf("name from archive: parse %s: %w", ManifestFile, err)
			}
			if err := m.Validate(); err != nil {
				return "", err
			}
			return m.Name, nil
		}
	}
	return "", fmt.Errorf("name from archive: %s not found in archive", ManifestFile)
}
