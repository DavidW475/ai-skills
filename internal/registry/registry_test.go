package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
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

// ---- newRepository ----

func TestNewRepository_validRef(t *testing.T) {
	setTempHome(t)
	_, err := newRepository("ghcr.io/myorg/skill:v1.0.0", true)
	if err != nil {
		t.Errorf("newRepository valid ref: %v", err)
	}
}

func TestNewRepository_invalidRef(t *testing.T) {
	setTempHome(t)
	_, err := newRepository("invalid ref!!!!", true)
	if err == nil {
		t.Error("newRepository invalid ref: expected error, got nil")
	}
}

// ---- Login ----

func TestLogin_success(t *testing.T) {
	setTempHome(t)
	ctx := context.Background()
	err := Login(ctx, "registry.example.com", "user", "password123")
	if err != nil {
		t.Errorf("Login() error: %v", err)
	}
}

// ---- fetchGitLabPage ----

func TestFetchGitLabPage_success(t *testing.T) {
	items := []repoItem{{Path: "ns/ansible"}, {Path: "ns/terraform"}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(items)
	}))
	defer ts.Close()

	ctx := context.Background()
	got, next, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "token123")
	if err != nil {
		t.Fatalf("fetchGitLabPage() error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
	if next != "" {
		t.Errorf("expected empty next, got %q", next)
	}
}

func TestFetchGitLabPage_withNextPage(t *testing.T) {
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next-Page", "2")
		json.NewEncoder(w).Encode([]repoItem{{Path: "ns/ansible"}})
	}))
	defer ts.Close()

	ctx := context.Background()
	got, next, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "tok")
	if err != nil {
		t.Fatalf("fetchGitLabPage() error: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("expected 1 item, got %d", len(got))
	}
	if !strings.Contains(next, "page=2") {
		t.Errorf("expected pagination in next URL, got %q", next)
	}
}

func TestFetchGitLabPage_unauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "bad")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got: %v", err)
	}
}

func TestFetchGitLabPage_forbidden(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "tok")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 error, got: %v", err)
	}
}

func TestFetchGitLabPage_serverError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "oops", http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "tok")
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 error, got: %v", err)
	}
}

func TestFetchGitLabPage_invalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	defer ts.Close()

	ctx := context.Background()
	_, _, err := fetchGitLabPage(ctx, ts.URL, ts.URL, "PRIVATE-TOKEN", "tok")
	if err == nil {
		t.Error("expected JSON parse error, got nil")
	}
}

// ---- collectAllPages ----

func TestCollectAllPages_singlePage(t *testing.T) {
	items := []repoItem{{Path: "ns/ansible"}, {Path: "ns/terraform"}}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(items)
	}))
	defer ts.Close()

	ctx := context.Background()
	got, err := collectAllPages(ctx, ts.URL, "PRIVATE-TOKEN", "tok")
	if err != nil {
		t.Fatalf("collectAllPages() error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d", len(got))
	}
}

func TestCollectAllPages_multiPage(t *testing.T) {
	page1 := []repoItem{{Path: "ns/ansible"}}
	page2 := []repoItem{{Path: "ns/terraform"}}
	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			json.NewEncoder(w).Encode(page2)
		} else {
			w.Header().Set("X-Next-Page", "2")
			json.NewEncoder(w).Encode(page1)
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	got, err := collectAllPages(ctx, ts.URL, "PRIVATE-TOKEN", "tok")
	if err != nil {
		t.Fatalf("collectAllPages() error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 items, got %d: %v", len(got), got)
	}
}

func TestCollectAllPages_error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	ctx := context.Background()
	_, err := collectAllPages(ctx, ts.URL, "PRIVATE-TOKEN", "tok")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ---- handleCatalogError ----

func TestHandleCatalogError_nonAuthError(t *testing.T) {
	setTempHome(t)
	cs := emptyCredStore(t)
	ctx := context.Background()
	catalogErr := errors.New("connection timeout")
	_, err := handleCatalogError(ctx, "myhost.example.com", "ns", cs, true, catalogErr)
	if err != catalogErr {
		t.Errorf("expected original catalogErr, got: %v", err)
	}
}

func TestHandleCatalogError_authError_nonRegistryHost(t *testing.T) {
	setTempHome(t)
	cs := emptyCredStore(t)
	ctx := context.Background()
	catalogErr := errors.New("401 Unauthorized")
	_, err := handleCatalogError(ctx, "ghcr.io", "myorg", cs, true, catalogErr)
	if err == nil || !strings.Contains(err.Error(), "catalog listing not available") {
		t.Errorf("expected 'catalog listing not available' error, got: %v", err)
	}
}

func TestHandleCatalogError_authError_registryHost_noCredentials(t *testing.T) {
	setTempHome(t)
	cs := emptyCredStore(t)
	ctx := context.Background()
	catalogErr := errors.New("401 Unauthorized")
	// host starts with "registry." so it tries the GitLab API path
	// with no credentials, listSkillsViaGitLabAPI returns "no credentials found"
	_, err := handleCatalogError(ctx, "registry.test.example.com", "ns", cs, true, catalogErr)
	if err == nil || !strings.Contains(err.Error(), "no credentials") {
		t.Errorf("expected 'no credentials' error, got: %v", err)
	}
}

func TestHandleCatalogError_authError_registryHost_emptyNamespace(t *testing.T) {
	// hasRegistryPrefix=true but namespace="" so it skips GitLab path
	setTempHome(t)
	cs := emptyCredStore(t)
	ctx := context.Background()
	catalogErr := errors.New("Unauthorized access")
	_, err := handleCatalogError(ctx, "registry.example.com", "", cs, true, catalogErr)
	// namespace="" means condition `namespace != ""` is false → falls through to isAuthErr branch
	if err == nil || !strings.Contains(err.Error(), "catalog listing not available") {
		t.Errorf("expected 'catalog listing not available' error, got: %v", err)
	}
}

// ---- ListTags ----

func newMockTagsServer(t *testing.T, repoPath string, tags []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"tags": tags})
			return
		}
		http.NotFound(w, r)
	}))
}

func TestListTags_success(t *testing.T) {
	setTempHome(t)
	ts := newMockTagsServer(t, "ns/ansible", []string{"v1.0.0", "v1.1.0"})
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	ref := host + "/ns/ansible"
	tags, err := ListTags(context.Background(), ref, true)
	if err != nil {
		t.Fatalf("ListTags() error: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(tags), tags)
	}
}

func TestListTags_noTag(t *testing.T) {
	setTempHome(t)
	// Tag-less ref is fine for ListTags (tags the repo path)
	ts := newMockTagsServer(t, "ns/ansible", nil)
	defer ts.Close()
	host := strings.TrimPrefix(ts.URL, "http://")
	tags, err := ListTags(context.Background(), host+"/ns/ansible", true)
	if err != nil {
		t.Fatalf("ListTags() error: %v", err)
	}
	if tags != nil {
		t.Errorf("expected nil tags, got %v", tags)
	}
}

func TestListTags_serverError(t *testing.T) {
	setTempHome(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	host := strings.TrimPrefix(ts.URL, "http://")
	_, err := ListTags(context.Background(), host+"/ns/ansible", true)
	if err == nil {
		t.Error("expected error from server error, got nil")
	}
}

// ---- ResolveDigest ----

// buildMockManifest returns (manifestJSON, manifestDigest, configDigest, layerDigest, layerBytes)
// for use in OCI mock servers.
func buildMockManifest(t *testing.T) ([]byte, digest.Digest, digest.Digest, digest.Digest, []byte) {
	t.Helper()

	emptyConfig := []byte("{}")
	configDig := digest.FromBytes(emptyConfig)

	// Build a minimal tar.gz layer
	var layerBuf bytes.Buffer
	gw := gzip.NewWriter(&layerBuf)
	tw := tar.NewWriter(gw)
	yamlContent := []byte("name: test-skill\nversion: v1.0.0\n")
	_ = tw.WriteHeader(&tar.Header{Name: "skill.yaml", Size: int64(len(yamlContent))})
	_, _ = tw.Write(yamlContent)
	_ = tw.Close()
	_ = gw.Close()
	layerBytes := layerBuf.Bytes()
	layerDig := digest.FromBytes(layerBytes)

	manifest := map[string]interface{}{
		"schemaVersion": 2,
		"mediaType":     ocispec.MediaTypeImageManifest,
		"config": ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageConfig,
			Digest:    configDig,
			Size:      int64(len(emptyConfig)),
		},
		"layers": []ocispec.Descriptor{{
			MediaType: MediaTypeSkillLayer,
			Digest:    layerDig,
			Size:      int64(len(layerBytes)),
		}},
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestDig := digest.FromBytes(manifestBytes)
	return manifestBytes, manifestDig, configDig, layerDig, layerBytes
}

func newMockOCIServer(t *testing.T, repoPath string) (*httptest.Server, []byte, digest.Digest, []byte) {
	t.Helper()
	manifestBytes, manifestDig, configDig, layerDig, layerBytes := buildMockManifest(t)
	emptyConfig := []byte("{}")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/manifests/"):
			ref := path[strings.LastIndex(path, "/")+1:]
			if ref == "v1.0.0" || ref == manifestDig.String() {
				w.Header().Set("Content-Type", ocispec.MediaTypeImageManifest)
				w.Header().Set("Docker-Content-Digest", manifestDig.String())
				w.Header().Set("Content-Length", strconv.Itoa(len(manifestBytes)))
				if r.Method != http.MethodHead {
					w.Write(manifestBytes)
				}
				return
			}
			http.NotFound(w, r)
		case strings.Contains(path, "/blobs/"):
			ref := path[strings.LastIndex(path, "/")+1:]
			switch ref {
			case configDig.String():
				w.Header().Set("Content-Type", ocispec.MediaTypeImageConfig)
				w.Header().Set("Docker-Content-Digest", configDig.String())
				w.Header().Set("Content-Length", strconv.Itoa(len(emptyConfig)))
				if r.Method != http.MethodHead {
					w.Write(emptyConfig)
				}
			case layerDig.String():
				w.Header().Set("Content-Type", MediaTypeSkillLayer)
				w.Header().Set("Docker-Content-Digest", layerDig.String())
				w.Header().Set("Content-Length", strconv.Itoa(len(layerBytes)))
				if r.Method != http.MethodHead {
					w.Write(layerBytes)
				}
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	return ts, manifestBytes, manifestDig, layerBytes
}

func TestResolveDigest_noTag(t *testing.T) {
	setTempHome(t)
	_, err := ResolveDigest(context.Background(), "ghcr.io/myorg/skill", true)
	if err == nil || !strings.Contains(err.Error(), "no tag") {
		t.Errorf("expected 'no tag' error, got: %v", err)
	}
}

func TestResolveDigest_success(t *testing.T) {
	setTempHome(t)
	ts, _, manifestDig, _ := newMockOCIServer(t, "ns/ansible")
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	ref := host + "/ns/ansible:v1.0.0"
	got, err := ResolveDigest(context.Background(), ref, true)
	if err != nil {
		t.Fatalf("ResolveDigest() error: %v", err)
	}
	if got != manifestDig.String() {
		t.Errorf("ResolveDigest() = %q, want %q", got, manifestDig.String())
	}
}

// ---- Pull ----

func TestPull_noTag(t *testing.T) {
	setTempHome(t)
	_, _, err := Pull(context.Background(), "ghcr.io/myorg/skill", true)
	if err == nil || !strings.Contains(err.Error(), "no tag") {
		t.Errorf("expected 'no tag' error, got: %v", err)
	}
}

func TestPull_success(t *testing.T) {
	setTempHome(t)
	ts, _, _, _ := newMockOCIServer(t, "ns/ansible")
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	ref := host + "/ns/ansible:v1.0.0"
	layerBytes, dig, err := Pull(context.Background(), ref, true)
	if err != nil {
		t.Fatalf("Pull() error: %v", err)
	}
	if len(layerBytes) == 0 {
		t.Error("Pull() returned empty layer bytes")
	}
	if dig == "" {
		t.Error("Pull() returned empty digest")
	}
}

// ---- ListSkills ----

func TestListSkills_invalidHost(t *testing.T) {
	setTempHome(t)
	_, err := ListSkills(context.Background(), "invalid host!!!!", true)
	if err == nil {
		t.Error("expected error for invalid host, got nil")
	}
}

func TestListSkills_catalogSuccess(t *testing.T) {
	setTempHome(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/_catalog" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"repositories": []string{"ns/ansible", "ns/terraform", "other/ignored"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	skills, err := ListSkills(context.Background(), host+"/ns", true)
	if err != nil {
		t.Fatalf("ListSkills() error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d: %v", len(skills), skills)
	}
}

func TestListSkills_catalogAuthError_returnsError(t *testing.T) {
	setTempHome(t)
	// Server returns 401 for catalog; host is non-registry → "catalog listing not available"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/_catalog" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="..."`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Auth challenge: return 401 so the client gives up
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	_, err := ListSkills(context.Background(), host+"/ns", true)
	if err == nil {
		t.Error("expected error from catalog auth error, got nil")
	}
}

// ---- listSkillsViaCatalog ----

func TestListSkillsViaCatalog_success(t *testing.T) {
	setTempHome(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"repositories": []string{"ns/ansible", "ns/terraform"},
		})
	}))
	defer ts.Close()

	uri := strings.TrimPrefix(ts.URL, "http://")
	reg, err := remote.NewRegistry(uri)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	reg.PlainHTTP = true
	reg.Client = &auth.Client{}

	skills, err := listSkillsViaCatalog(context.Background(), reg, "ns")
	if err != nil {
		t.Fatalf("listSkillsViaCatalog() error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d: %v", len(skills), skills)
	}
}
