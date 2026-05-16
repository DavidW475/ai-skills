package installer

import (
	"context"
	"strings"
	"testing"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// ---- Install ----

func TestInstall_emptyLockfile(t *testing.T) {
	setTempHome(t)

	results, err := Install(context.Background(), Options{})
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected nil/empty results for empty lockfile, got %v", results)
	}
}

// ---- InstallOne ----

func TestInstallOne_noSources(t *testing.T) {
	setTempHome(t)

	_, err := InstallOne(context.Background(), "ansible", "", Options{})
	if err == nil {
		t.Fatal("InstallOne() should error when no sources are configured")
	}
	if !strings.Contains(err.Error(), "no sources configured") {
		t.Errorf("expected 'no sources configured' error, got: %v", err)
	}
}
