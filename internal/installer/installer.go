package installer

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/DavidW475/ai-skills/internal/config"
	"github.com/DavidW475/ai-skills/internal/lockfile"
	"github.com/DavidW475/ai-skills/internal/registry"
	"github.com/DavidW475/ai-skills/internal/resolver"
	"github.com/DavidW475/ai-skills/internal/skill"
	"github.com/DavidW475/ai-skills/internal/sources"
)

// Options controls install behaviour.
type Options struct {
	PlainHTTP bool
	// Force re-installs even if digest matches the installed index.
	Force bool
}

// Result describes the outcome for a single skill.
type Result struct {
	Name    string `json:"name"`
	Ref     string `json:"ref"`
	Digest  string `json:"digest"`
	Path    string `json:"path"`
	Skipped bool   `json:"skipped"` // true when already up-to-date
}

// Install re-resolves and updates all skills listed in ~/.ai-skills/installed.yaml.
// Skills whose remote digest matches the installed index are skipped.
// Used by the update command.
func Install(ctx context.Context, opts Options) ([]Result, error) {
	lf, err := lockfile.Load()
	if err != nil {
		return nil, err
	}
	if len(lf.Skills) == 0 {
		return nil, nil
	}

	sf, err := sources.Load()
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, e := range lf.Skills {
		r, err := installByName(ctx, e.Name, "", sf.Sources, lf, opts)
		if err != nil {
			return results, fmt.Errorf("update %s: %w", e.Name, err)
		}
		results = append(results, r)
	}

	return results, lockfile.Save(lf)
}

// InstallOne installs a single skill (by name+version) into the global skills
// directory and updates ~/.ai-skills/installed.yaml.
// Sources are read from ~/.ai-skills/sources.
func InstallOne(ctx context.Context, name, version string, opts Options) (Result, error) {
	sf, err := sources.Load()
	if err != nil {
		return Result{}, err
	}

	lf, err := lockfile.Load()
	if err != nil {
		return Result{}, err
	}

	r, err := installByName(ctx, name, version, sf.Sources, lf, opts)
	if err != nil {
		return Result{}, err
	}
	return r, lockfile.Save(lf)
}

func installByName(ctx context.Context, name, version string, srcs []string, lf *lockfile.LockFile, opts Options) (Result, error) {
	ref, remoteDigest, err := resolver.Resolve(ctx, srcs, name, version, opts.PlainHTTP)
	if err != nil {
		return Result{}, err
	}

	// Skip if already up-to-date
	if !opts.Force {
		if e := lf.Find(name); e != nil && e.Digest == remoteDigest {
			return Result{Name: name, Ref: ref, Digest: remoteDigest, Path: e.Installed, Skipped: true}, nil
		}
	}

	tarBytes, manifestDigest, err := registry.Pull(ctx, ref, opts.PlainHTTP)
	if err != nil {
		return Result{}, err
	}

	skillName, err := skill.NameFromArchive(bytes.NewReader(tarBytes))
	if err != nil {
		return Result{}, fmt.Errorf("cannot determine skill name from archive: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return Result{}, err
	}
	baseDir, err := cfg.SkillsInstallDir()
	if err != nil {
		return Result{}, err
	}

	destDir := filepath.Join(baseDir, skillName)
	if err := skill.Unpack(bytes.NewReader(tarBytes), destDir); err != nil {
		return Result{}, fmt.Errorf("unpack: %w", err)
	}

	lf.Upsert(lockfile.Entry{
		Name:      name,
		Resolved:  ref,
		Digest:    manifestDigest,
		Installed: destDir,
	})

	return Result{Name: name, Ref: ref, Digest: manifestDigest, Path: destDir}, nil
}
