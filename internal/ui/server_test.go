package ui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DavidW475/ai-skills/internal/config"
	"github.com/DavidW475/ai-skills/internal/lockfile"
	"github.com/DavidW475/ai-skills/internal/sources"
)

// tempHome sets HOME to a fresh temp dir for the duration of the test so that
// lockfile.Load() and sources.Load() operate on isolated, empty state.
func tempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func newServer() *server { return &server{plainHTTP: true} }

// saveSources writes a sources file to the current temp HOME.
func saveSources(t *testing.T, srcs ...string) {
	t.Helper()
	sf := &sources.File{Sources: srcs}
	if err := sources.Save(sf); err != nil {
		t.Fatalf("saveSources: %v", err)
	}
}

// saveLockfile writes a lockfile to the current temp HOME.
func saveLockfile(t *testing.T, entries ...lockfile.Entry) {
	t.Helper()
	lf := &lockfile.LockFile{Skills: entries}
	if err := lockfile.Save(lf); err != nil {
		t.Fatalf("saveLockfile: %v", err)
	}
}

// decodeJSON decodes the response body into v.
func decodeJSON(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decodeJSON: %v (body: %s)", err, body)
	}
}

// ---- installedVersion ----

func TestInstalledVersion(t *testing.T) {
	cases := []struct {
		resolved string
		want     string
	}{
		{"registry.example.com/ns/skill:v1.2.3", "v1.2.3"},
		{"host/path/name:latest", "latest"},
		{"no-colon-here", ""},
		{"", ""},
	}
	for _, tc := range cases {
		got := installedVersion(tc.resolved)
		if got != tc.want {
			t.Errorf("installedVersion(%q) = %q, want %q", tc.resolved, got, tc.want)
		}
	}
}

// ---- handleInstalled ----

func TestHandleInstalled_emptyLockfile(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/installed", nil)
	w := httptest.NewRecorder()
	srv.handleInstalled(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var skills []any
	decodeJSON(t, w.Body.Bytes(), &skills)
	if len(skills) != 0 {
		t.Errorf("expected empty list, got %v", skills)
	}
}

func TestHandleInstalled_withSkills(t *testing.T) {
	tempHome(t)
	saveLockfile(t,
		lockfile.Entry{Name: "ansible", Resolved: "r/ns/ansible:v1.0.0", Digest: "d1", Installed: "/i/ansible"},
		lockfile.Entry{Name: "terraform", Resolved: "r/ns/terraform:v2.0.0", Digest: "d2", Installed: "/i/terraform"},
	)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/installed", nil)
	w := httptest.NewRecorder()
	srv.handleInstalled(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var skills []map[string]any
	decodeJSON(t, w.Body.Bytes(), &skills)
	if len(skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(skills))
	}
}

// ---- handleSourcesList ----

func TestHandleSourcesList_empty(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/sources/list", nil)
	w := httptest.NewRecorder()
	srv.handleSourcesList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var list []string
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("expected empty list, got %v", list)
	}
}

func TestHandleSourcesList_withSources(t *testing.T) {
	tempHome(t)
	saveSources(t, "registry.example.com/ns")
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/sources/list", nil)
	w := httptest.NewRecorder()
	srv.handleSourcesList(w, req)

	var list []string
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list) != 1 || list[0] != "registry.example.com/ns" {
		t.Errorf("unexpected list: %v", list)
	}
}

// ---- handleSourcesAdd ----

func TestHandleSourcesAdd_wrongMethod(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/sources/add", nil)
	w := httptest.NewRecorder()
	srv.handleSourcesAdd(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSourcesAdd_emptySource(t *testing.T) {
	tempHome(t)
	srv := newServer()

	body := `{"Source":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/add", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSourcesAdd(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleSourcesAdd_success(t *testing.T) {
	tempHome(t)
	srv := newServer()

	body := `{"Source":"registry.example.com/ns"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/add", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSourcesAdd(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var list []string
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list) != 1 || list[0] != "registry.example.com/ns" {
		t.Errorf("unexpected list: %v", list)
	}

	// Verify it was persisted
	sf, _ := sources.Load()
	if len(sf.Sources) != 1 {
		t.Errorf("source not persisted, got %v", sf.Sources)
	}
}

// ---- handleSourcesRemove ----

func TestHandleSourcesRemove_wrongMethod(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/sources/remove", nil)
	w := httptest.NewRecorder()
	srv.handleSourcesRemove(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleSourcesRemove_success(t *testing.T) {
	tempHome(t)
	saveSources(t, "registry.example.com/ns")
	srv := newServer()

	body := `{"Source":"registry.example.com/ns"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sources/remove", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleSourcesRemove(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var list []string
	decodeJSON(t, w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("expected empty list after remove, got %v", list)
	}
}

// ---- handleUninstall ----

func TestHandleUninstall_wrongMethod(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/uninstall", nil)
	w := httptest.NewRecorder()
	srv.handleUninstall(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleUninstall_emptyName(t *testing.T) {
	tempHome(t)
	srv := newServer()

	body := `{"Name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleUninstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleUninstall_notInstalled(t *testing.T) {
	tempHome(t)
	srv := newServer()

	body := `{"Name":"nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleUninstall(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleUninstall_success(t *testing.T) {
	tempHome(t)
	// Create a real installed directory so RemoveAll can succeed.
	home := filepath.Join(t.TempDir()) // already set via t.Setenv above but we need the actual value
	_ = home
	saveLockfile(t, lockfile.Entry{Name: "ansible", Resolved: "r:v1", Digest: "d", Installed: ""})
	srv := newServer()

	body := `{"Name":"ansible"}`
	req := httptest.NewRequest(http.MethodPost, "/api/uninstall", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleUninstall(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	var resp map[string]string
	decodeJSON(t, w.Body.Bytes(), &resp)
	if resp["removed"] != "ansible" {
		t.Errorf("resp = %v, want removed=ansible", resp)
	}
}

// ---- handleInstall ----

func TestHandleInstall_wrongMethod(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/install", nil)
	w := httptest.NewRecorder()
	srv.handleInstall(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestHandleInstall_emptyName(t *testing.T) {
	tempHome(t)
	srv := newServer()

	body := `{"Name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/install", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.handleInstall(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- handleUpdateAll ----

func TestHandleUpdateAll_wrongMethod(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/update-all", nil)
	w := httptest.NewRecorder()
	srv.handleUpdateAll(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

// ---- handleCheckUpdate ----

func TestHandleCheckUpdate_missingName(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/check-update", nil)
	w := httptest.NewRecorder()
	srv.handleCheckUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleCheckUpdate_notInstalled(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/check-update?name=nonexistent", nil)
	w := httptest.NewRecorder()
	srv.handleCheckUpdate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestHandleCheckUpdate_noSources(t *testing.T) {
	tempHome(t)
	saveLockfile(t, lockfile.Entry{Name: "ansible", Resolved: "r:v1.0.0", Digest: "d", Installed: ""})
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/check-update?name=ansible", nil)
	w := httptest.NewRecorder()
	srv.handleCheckUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp map[string]any
	decodeJSON(t, w.Body.Bytes(), &resp)
	if resp["current"] != "v1.0.0" {
		t.Errorf("current = %v, want v1.0.0", resp["current"])
	}
}

// ---- handleVersions ----

func TestHandleVersions_missingName(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/versions", nil)
	w := httptest.NewRecorder()
	srv.handleVersions(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestHandleVersions_noSources(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/versions?name=ansible", nil)
	w := httptest.NewRecorder()
	srv.handleVersions(w, req)

	// Empty sources → empty result list with 200
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var results []any
	decodeJSON(t, w.Body.Bytes(), &results)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}

// ---- handleAvailable ----

func TestHandleAvailable_noSources(t *testing.T) {
	tempHome(t)
	srv := newServer()

	req := httptest.NewRequest(http.MethodGet, "/api/available", nil)
	w := httptest.NewRecorder()
	srv.handleAvailable(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var results []any
	decodeJSON(t, w.Body.Bytes(), &results)
	if len(results) != 0 {
		t.Errorf("expected empty results, got %v", results)
	}
}

// ---- writeJSON / writeError via helper ----

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, map[string]string{"key": "value"})

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"key"`)) {
		t.Errorf("response body missing expected key: %s", w.Body.String())
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, "something went wrong", http.StatusInternalServerError)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("something went wrong")) {
		t.Errorf("body missing error message: %s", w.Body.String())
	}
}

// Keep unused import satisfied (config is needed for the path helper)
var _ = config.DirName
