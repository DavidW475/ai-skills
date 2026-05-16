package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ---- UserDir ----

func TestUserDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir, err := UserDir()
	if err != nil {
		t.Fatalf("UserDir() error: %v", err)
	}
	want := filepath.Join(home, DirName)
	if dir != want {
		t.Errorf("UserDir() = %q, want %q", dir, want)
	}
}

// ---- Load ----

func TestLoad_missing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.SkillsDir != "" {
		t.Errorf("expected empty config, got %+v", cfg)
	}
}

func TestLoad_valid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "skills_dir: ~/custom/skills\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.SkillsDir != "~/custom/skills" {
		t.Errorf("SkillsDir = %q, want %q", cfg.SkillsDir, "~/custom/skills")
	}
}

func TestLoad_invalidYAML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dir := filepath.Join(home, DirName)
	os.MkdirAll(dir, 0o755)                                                              //nolint:errcheck
	os.WriteFile(filepath.Join(dir, ConfigFile), []byte("skills_dir: [unclosed"), 0o644) //nolint:errcheck

	if _, err := Load(); err == nil {
		t.Error("Load() should error on invalid YAML")
	}
}

// ---- Save / round-trip ----

func TestSave_roundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := &Config{SkillsDir: "~/my/skills"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save() error: %v", err)
	}
	if loaded.SkillsDir != cfg.SkillsDir {
		t.Errorf("SkillsDir = %q, want %q", loaded.SkillsDir, cfg.SkillsDir)
	}
}

// ---- SkillsInstallDir ----

func TestSkillsInstallDir_default(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{}
	dir, err := cfg.SkillsInstallDir()
	if err != nil {
		t.Fatalf("SkillsInstallDir() error: %v", err)
	}
	want := filepath.Join(home, defaultSkillsSubdir)
	if dir != want {
		t.Errorf("SkillsInstallDir() = %q, want %q", dir, want)
	}
}

func TestSkillsInstallDir_tilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := &Config{SkillsDir: "~/myskills"}
	dir, err := cfg.SkillsInstallDir()
	if err != nil {
		t.Fatalf("SkillsInstallDir() error: %v", err)
	}
	want := filepath.Join(home, "myskills")
	if dir != want {
		t.Errorf("SkillsInstallDir() = %q, want %q", dir, want)
	}
}

func TestSkillsInstallDir_absolutePath(t *testing.T) {
	cfg := &Config{SkillsDir: "/absolute/path"}
	dir, err := cfg.SkillsInstallDir()
	if err != nil {
		t.Fatalf("SkillsInstallDir() error: %v", err)
	}
	if dir != "/absolute/path" {
		t.Errorf("SkillsInstallDir() = %q, want /absolute/path", dir)
	}
}

// ---- expandHome ----

func TestExpandHome_tilde(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := expandHome("~/foo/bar")
	if err != nil {
		t.Fatalf("expandHome() error: %v", err)
	}
	want := filepath.Join(home, "foo/bar")
	if got != want {
		t.Errorf("expandHome(~/foo/bar) = %q, want %q", got, want)
	}
}

func TestExpandHome_noTilde(t *testing.T) {
	got, err := expandHome("/absolute/path")
	if err != nil {
		t.Fatalf("expandHome() error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("expandHome() = %q, want /absolute/path", got)
	}
}

func TestExpandHome_tildeOnly(t *testing.T) {
	// "~" alone — len == 1, so NOT expanded (condition: len > 1)
	got, err := expandHome("~")
	if err != nil {
		t.Fatalf("expandHome() error: %v", err)
	}
	if got != "~" {
		t.Errorf("expandHome('~') = %q, want '~'", got)
	}
}

func TestExpandHome_regularPath(t *testing.T) {
	got, err := expandHome("relative/path")
	if err != nil {
		t.Fatalf("expandHome() error: %v", err)
	}
	if got != "relative/path" {
		t.Errorf("expandHome() = %q, want relative/path", got)
	}
}
