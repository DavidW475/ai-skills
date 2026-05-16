package aiskills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DavidW475/ai-skills/internal/installer"
	"github.com/DavidW475/ai-skills/internal/lockfile"
	"github.com/DavidW475/ai-skills/internal/sources"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// ---- parseNameVersion ----

func TestParseNameVersion_noVersion(t *testing.T) {
	name, version := parseNameVersion("ansible")
	if name != "ansible" || version != "" {
		t.Errorf("got (%q, %q), want (ansible, \"\")", name, version)
	}
}

func TestParseNameVersion_withVersion(t *testing.T) {
	name, version := parseNameVersion("ansible@v1.2.3")
	if name != "ansible" || version != "v1.2.3" {
		t.Errorf("got (%q, %q), want (ansible, v1.2.3)", name, version)
	}
}

func TestParseNameVersion_bareVersion(t *testing.T) {
	name, version := parseNameVersion("ansible@1.2.3")
	if name != "ansible" || version != "1.2.3" {
		t.Errorf("got (%q, %q), want (ansible, 1.2.3)", name, version)
	}
}

func TestParseNameVersion_lastAtSplit(t *testing.T) {
	// LastIndex means the last '@' is the split point
	name, version := parseNameVersion("my@skill@v1.0.0")
	if name != "my@skill" || version != "v1.0.0" {
		t.Errorf("got (%q, %q), want (my@skill, v1.0.0)", name, version)
	}
}

// ---- printResult ----

func TestPrintResult_skipped(t *testing.T) {
	cmd := newListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	printResult(cmd, installer.Result{Name: "ansible", Skipped: true})
	if !strings.Contains(out.String(), "ansible") || !strings.Contains(out.String(), "up-to-date") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestPrintResult_downloaded(t *testing.T) {
	cmd := newListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	printResult(cmd, installer.Result{Name: "ansible", Ref: "r:v1", Path: "/p/ansible", Skipped: false})
	if !strings.Contains(out.String(), "ansible") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// ---- Command constructors ----

func TestNewCmdConstructors(t *testing.T) {
	constructors := []func(){
		func() { newInstallCmd() },
		func() { newListCmd() },
		func() { newSourceCmd() },
		func() { newSourceAddCmd() },
		func() { newSourceRemoveCmd() },
		func() { newSourceListCmd() },
		func() { newSearchCmd() },
		func() { newUpdateCmd() },
		func() { newVersionsCmd() },
		func() { newInitCmd() },
		func() { newUninstallCmd() },
		func() { newUICmd() },
		func() { newPublishCmd() },
		func() { newLoginCmd() },
	}
	for _, f := range constructors {
		f() // must not panic
	}
}

// ---- list ----

func TestListCmd_empty(t *testing.T) {
	setTempHome(t)
	cmd := newListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "no skills installed") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestListCmd_withSkills(t *testing.T) {
	setTempHome(t)
	lf := &lockfile.LockFile{}
	lf.Upsert(lockfile.Entry{Name: "ansible", Resolved: "r:v1.0.0", Digest: "d", Installed: "/i"})
	lockfile.Save(lf) //nolint:errcheck

	cmd := newListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "ansible") {
		t.Errorf("expected ansible in output: %q", out.String())
	}
}

// ---- source list ----

func TestSourceListCmd_empty(t *testing.T) {
	setTempHome(t)
	cmd := newSourceListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "No sources configured") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestSourceListCmd_withSources(t *testing.T) {
	setTempHome(t)
	sf := &sources.File{Sources: []string{"registry.example.com/ns"}}
	sources.Save(sf) //nolint:errcheck

	cmd := newSourceListCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "registry.example.com/ns") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// ---- source add ----

func TestSourceAddCmd_new(t *testing.T) {
	setTempHome(t)
	cmd := newSourceAddCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{"registry.example.com/ns"}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "added source") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestSourceAddCmd_duplicate(t *testing.T) {
	setTempHome(t)
	sf := &sources.File{Sources: []string{"registry.example.com/ns"}}
	sources.Save(sf) //nolint:errcheck

	cmd := newSourceAddCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{"registry.example.com/ns"}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "already in sources") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// ---- source remove ----

func TestSourceRemoveCmd_existing(t *testing.T) {
	setTempHome(t)
	sf := &sources.File{Sources: []string{"registry.example.com/ns"}}
	sources.Save(sf) //nolint:errcheck

	cmd := newSourceRemoveCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{"registry.example.com/ns"}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "removed source") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestSourceRemoveCmd_notFound(t *testing.T) {
	setTempHome(t)
	cmd := newSourceRemoveCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{"registry.example.com/ns"}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if !strings.Contains(out.String(), "not in sources") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

// ---- init ----

func TestInitCmd_success(t *testing.T) {
	setTempHome(t)
	dir := t.TempDir()
	t.Chdir(dir)

	cmd := newInitCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := cmd.RunE(cmd, []string{"test-skill"}); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "test-skill", "skill.yaml")); err != nil {
		t.Errorf("skill.yaml not created: %v", err)
	}
	if !strings.Contains(out.String(), "Created skill") {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestInitCmd_invalidName(t *testing.T) {
	setTempHome(t)
	t.Chdir(t.TempDir())

	cmd := newInitCmd()
	if err := cmd.RunE(cmd, []string{"Invalid Name!"}); err == nil {
		t.Error("RunE should fail for invalid skill name")
	}
}

// ---- uninstall ----

func TestRunUninstall_notInstalled(t *testing.T) {
	setTempHome(t)
	cmd := newUninstallCmd()
	errBuf := &bytes.Buffer{}
	cmd.SetErr(errBuf)
	if err := runUninstall(cmd, []string{"nonexistent"}, false); err == nil {
		t.Error("runUninstall should return error when skill is not installed")
	}
}

func TestRunUninstall_success(t *testing.T) {
	setTempHome(t)
	lf := &lockfile.LockFile{}
	lf.Upsert(lockfile.Entry{Name: "ansible", Resolved: "r:v1", Digest: "d", Installed: ""})
	lockfile.Save(lf) //nolint:errcheck

	cmd := newUninstallCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := runUninstall(cmd, []string{"ansible"}, false); err != nil {
		t.Fatalf("runUninstall error: %v", err)
	}
	if !strings.Contains(out.String(), "removed ansible") {
		t.Errorf("unexpected output: %q", out.String())
	}
	loaded, _ := lockfile.Load()
	if loaded.Find("ansible") != nil {
		t.Error("ansible still in lockfile after uninstall")
	}
}

func TestRunUninstall_keepFiles(t *testing.T) {
	setTempHome(t)
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "ansible")
	os.MkdirAll(skillDir, 0o755) //nolint:errcheck

	lf := &lockfile.LockFile{}
	lf.Upsert(lockfile.Entry{Name: "ansible", Resolved: "r:v1", Digest: "d", Installed: skillDir})
	lockfile.Save(lf) //nolint:errcheck

	cmd := newUninstallCmd()
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	if err := runUninstall(cmd, []string{"ansible"}, true); err != nil {
		t.Fatalf("runUninstall error: %v", err)
	}
	if _, err := os.Stat(skillDir); err != nil {
		t.Errorf("skill dir should still exist with --keep-files: %v", err)
	}
}

func TestRunUninstall_removesFiles(t *testing.T) {
	setTempHome(t)
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "ansible")
	os.MkdirAll(skillDir, 0o755) //nolint:errcheck

	lf := &lockfile.LockFile{}
	lf.Upsert(lockfile.Entry{Name: "ansible", Resolved: "r:v1", Digest: "d", Installed: skillDir})
	lockfile.Save(lf) //nolint:errcheck

	cmd := newUninstallCmd()
	cmd.SetOut(&bytes.Buffer{})
	if err := runUninstall(cmd, []string{"ansible"}, false); err != nil {
		t.Fatalf("runUninstall error: %v", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Errorf("skill dir should be removed, err: %v", err)
	}
}

// ---- search ----

func TestRunSearch_noSources(t *testing.T) {
	setTempHome(t)
	cmd := newSearchCmd()
	cmd.SetOut(&bytes.Buffer{})
	err := runSearch(cmd, false)
	if err == nil || !strings.Contains(err.Error(), "no sources configured") {
		t.Errorf("expected 'no sources configured' error, got: %v", err)
	}
}

// ---- versions ----

func TestVersionsCmd_noSources(t *testing.T) {
	setTempHome(t)
	cmd := newVersionsCmd()
	cmd.SetOut(&bytes.Buffer{})
	err := cmd.RunE(cmd, []string{"ansible"})
	if err == nil || !strings.Contains(err.Error(), "no sources configured") {
		t.Errorf("expected 'no sources configured' error, got: %v", err)
	}
}
