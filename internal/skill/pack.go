package skill

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Pack creates a .tar.gz archive of the skill directory and writes it to dst.
// The archive contains all files relative to dir (e.g. skill.yaml, SKILL.md, assets/).
func Pack(dir string, dst io.Writer) error {
	if _, err := LoadManifest(dir); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, SkillFile)); os.IsNotExist(err) {
		return fmt.Errorf("pack: %s not found in %s", SkillFile, dir)
	}

	gz := gzip.NewWriter(dst)
	tw := tar.NewWriter(gz)

	err := filepath.Walk(dir, makePackWalkFn(dir, tw))
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("pack: tar close: %w", err)
	}
	return gz.Close()
}

func makePackWalkFn(dir string, tw *tar.Writer) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		// Skip the root directory entry itself
		if rel == "." {
			return nil
		}
		// Normalize to forward slashes for cross-platform archives
		rel = filepath.ToSlash(rel)

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = rel
		if info.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return writePackEntry(tw, path)
	}
}

func writePackEntry(tw *tar.Writer, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(tw, f)
	closeErr := f.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Unpack extracts a .tar.gz skill archive from src into dir.
// It strips any top-level directory prefix that matches a skill name.
func Unpack(src io.Reader, dir string) error {
	gr, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("unpack: not a gzip stream: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("unpack: %w", err)
		}
		if err := unpackEntry(tr, hdr, dir); err != nil {
			return err
		}
	}
	return nil
}

func unpackEntry(tr *tar.Reader, hdr *tar.Header, dir string) error {
	// Security: reject absolute paths and path traversal
	name := filepath.FromSlash(hdr.Name)
	if filepath.IsAbs(name) || strings.Contains(name, "..") {
		return fmt.Errorf("unpack: unsafe path in archive: %q", hdr.Name)
	}

	target := filepath.Join(dir, name)

	switch hdr.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		return unpackFile(tr, target)
	}
	return nil
}

func unpackFile(tr *tar.Reader, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	// Limit file permissions to owner read/write
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	// Limit read to 10 MB per file to prevent decompression bombs
	_, copyErr := io.Copy(f, io.LimitReader(tr, 10<<20))
	f.Close()
	if copyErr != nil {
		return fmt.Errorf("unpack: writing %s: %w", target, copyErr)
	}
	return nil
}
