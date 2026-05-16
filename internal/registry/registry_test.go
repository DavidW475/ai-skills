package registry

import (
	"errors"
	"strings"
	"testing"

	"oras.land/oras-go/v2/registry/remote/credentials"
)

func setTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func emptyCredStore(t *testing.T) credentials.Store {
	t.Helper()
	setTempHome(t)
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		t.Fatalf("NewStoreFromDocker: %v", err)
	}
	return store
}

// ---- wrapAuthError ----

func TestWrapAuthError_nil(t *testing.T) {
	if err := wrapAuthError(nil, "registry.example.com/ns/skill:v1"); err != nil {
		t.Errorf("wrapAuthError(nil, ...) = %v, want nil", err)
	}
}

func TestWrapAuthError_401(t *testing.T) {
	base := errors.New("request failed: 401 Unauthorized")
	err := wrapAuthError(base, "ghcr.io/myorg/skill:v1")
	if err == nil {
		t.Fatal("expected wrapped error, got nil")
	}
	if !strings.Contains(err.Error(), "ai-skills login") {
		t.Errorf("expected login hint in error, got: %s", err.Error())
	}
}

func TestWrapAuthError_Unauthorized(t *testing.T) {
	base := errors.New("Unauthorized")
	err := wrapAuthError(base, "registry.gitlab.com/ns/skill:v1")
	if err == nil {
		t.Fatal("expected wrapped error, got nil")
	}
	if !strings.Contains(err.Error(), "ai-skills login") {
		t.Errorf("expected login hint in error, got: %s", err.Error())
	}
}

func TestWrapAuthError_otherError(t *testing.T) {
	base := errors.New("connection refused")
	err := wrapAuthError(base, "registry.example.com/ns/skill:v1")
	if err != base {
		t.Errorf("wrapAuthError should return original error for non-auth errors, got: %v", err)
	}
}

// ---- registryFromRef ----

func TestRegistryFromRef_ghcr(t *testing.T) {
	got := registryFromRef("ghcr.io/myorg/my-skill:v1.0.0")
	if got != "ghcr.io" {
		t.Errorf("registryFromRef() = %q, want ghcr.io", got)
	}
}

func TestRegistryFromRef_docker(t *testing.T) {
	got := registryFromRef("registry.hub.docker.com/library/alpine:3.21")
	if !strings.Contains(got, "docker.com") && !strings.Contains(got, "registry") {
		t.Errorf("registryFromRef() = %q, unexpected value", got)
	}
}

func TestRegistryFromRef_invalid(t *testing.T) {
	// Invalid ref — should return the input ref itself
	got := registryFromRef("not a valid ref !!!")
	if got == "" {
		t.Error("registryFromRef should return non-empty string for invalid refs")
	}
}

// ---- extractSkillNames ----

func TestExtractSkillNames_basic(t *testing.T) {
	items := []repoItem{
		{Path: "ns/ansible"},
		{Path: "ns/terraform"},
		{Path: "other-ns/kubectl"}, // different namespace — excluded
		{Path: "ns/"},              // empty name — excluded
	}
	got := extractSkillNames(items, "ns")
	if len(got) != 2 {
		t.Fatalf("expected 2 skills, got %d: %v", len(got), got)
	}
	if got[0] != "ansible" || got[1] != "terraform" {
		t.Errorf("unexpected skills: %v", got)
	}
}

func TestExtractSkillNames_empty(t *testing.T) {
	got := extractSkillNames(nil, "ns")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestExtractSkillNames_noMatch(t *testing.T) {
	items := []repoItem{{Path: "other/skill"}}
	got := extractSkillNames(items, "ns")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

// ---- extractTag ----

func TestExtractTag_withTag(t *testing.T) {
	got := extractTag("ghcr.io/myorg/skill:v1.2.3")
	if got != "v1.2.3" {
		t.Errorf("extractTag() = %q, want v1.2.3", got)
	}
}

func TestExtractTag_withDigest(t *testing.T) {
	ref := "ghcr.io/myorg/skill@sha256:abc123"
	got := extractTag(ref)
	if got != "sha256:abc123" {
		t.Errorf("extractTag() = %q, want sha256:abc123", got)
	}
}

func TestExtractTag_noTag(t *testing.T) {
	got := extractTag("ghcr.io/myorg/skill")
	if got != "" {
		t.Errorf("extractTag() = %q, want empty string", got)
	}
}

func TestExtractTag_latest(t *testing.T) {
	got := extractTag("ghcr.io/myorg/skill:latest")
	if got != "latest" {
		t.Errorf("extractTag() = %q, want latest", got)
	}
}
