package resolver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- parseSemVer ----

func TestParseSemVer_valid(t *testing.T) {
	cases := []struct {
		tag   string
		major int
		minor int
		patch int
		pre   string
	}{
		{"1.2.3", 1, 2, 3, ""},
		{"v1.2.3", 1, 2, 3, ""},
		{"0.0.1", 0, 0, 1, ""},
		{"v2.10.99-alpha.1", 2, 10, 99, "alpha.1"},
		{"1.0.0-rc.2", 1, 0, 0, "rc.2"},
	}
	for _, tc := range cases {
		sv, ok := parseSemVer(tc.tag)
		if !ok {
			t.Errorf("parseSemVer(%q) returned ok=false, want true", tc.tag)
			continue
		}
		if sv.major != tc.major || sv.minor != tc.minor || sv.patch != tc.patch || sv.pre != tc.pre {
			t.Errorf("parseSemVer(%q) = {%d %d %d %q}, want {%d %d %d %q}",
				tc.tag, sv.major, sv.minor, sv.patch, sv.pre,
				tc.major, tc.minor, tc.patch, tc.pre)
		}
	}
}

func TestParseSemVer_invalid(t *testing.T) {
	invalid := []string{"", "latest", "1.2", "1.2.x", "abc", "v1.2", "1.2.3.4"}
	for _, tag := range invalid {
		if _, ok := parseSemVer(tag); ok {
			t.Errorf("parseSemVer(%q) returned ok=true, want false", tag)
		}
	}
}

// ---- IsNewer ----

func TestIsNewer(t *testing.T) {
	cases := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"v1.2.4", "v1.2.3", true},
		{"1.3.0", "1.2.9", true},
		{"2.0.0", "1.9.9", true},
		{"v1.2.3", "v1.2.3", false},      // equal
		{"v1.2.2", "v1.2.3", false},      // older
		{"1.0.0", "1.0.0-alpha.1", true}, // stable > pre-release
		{"1.0.0-beta.1", "1.0.0", false}, // pre-release < stable
		{"notvalid", "1.0.0", false},
		{"1.0.0", "notvalid", false},
	}
	for _, tc := range cases {
		got := IsNewer(tc.candidate, tc.current)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.candidate, tc.current, got, tc.want)
		}
	}
}

// ---- LatestTag ----

func TestLatestTag(t *testing.T) {
	cases := []struct {
		tags []string
		want string
	}{
		{[]string{"v1.0.0", "v1.2.0", "v1.1.5"}, "v1.2.0"},
		{[]string{"0.0.1", "0.0.2", "0.1.0"}, "0.1.0"},
		{[]string{"latest", "stable", "v1.0.0"}, "v1.0.0"},             // ignores non-semver
		{[]string{"v1.0.0-alpha.1", "v1.0.0"}, "v1.0.0"},               // stable wins over pre
		{[]string{"v1.0.0-alpha.1", "v1.0.0-beta.1"}, "v1.0.0-beta.1"}, // beta > alpha
		{[]string{"latest", "stable"}, ""},                             // no semver at all
		{[]string{}, ""},
	}
	for _, tc := range cases {
		got := LatestTag(tc.tags)
		if got != tc.want {
			t.Errorf("LatestTag(%v) = %q, want %q", tc.tags, got, tc.want)
		}
	}
}

// ---- compareSemVer ----

func TestCompareSemVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int // >0, 0, <0
	}{
		{"v2.0.0", "v1.9.9", 1},
		{"v1.9.9", "v2.0.0", -1},
		{"v1.2.3", "v1.2.3", 0},
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0-beta", "v1.0.0-alpha", 1}, // string compare on pre
	}
	for _, tc := range cases {
		a, _ := parseSemVer(tc.a)
		b, _ := parseSemVer(tc.b)
		got := compareSemVer(a, b)
		switch {
		case tc.want > 0 && got <= 0:
			t.Errorf("compareSemVer(%q, %q) = %d, want >0", tc.a, tc.b, got)
		case tc.want < 0 && got >= 0:
			t.Errorf("compareSemVer(%q, %q) = %d, want <0", tc.a, tc.b, got)
		case tc.want == 0 && got != 0:
			t.Errorf("compareSemVer(%q, %q) = %d, want 0", tc.a, tc.b, got)
		}
	}
}

// ---- tagsForVersion ----

func TestTagsForVersion_explicitWithV(t *testing.T) {
	got, err := tagsForVersion(nil, "example.com/ns/skill", "v1.2.3", true)
	if err != nil {
		t.Fatalf("tagsForVersion() error: %v", err)
	}
	if len(got) != 2 || got[0] != "v1.2.3" || got[1] != "1.2.3" {
		t.Errorf("tagsForVersion() = %v, want [v1.2.3 1.2.3]", got)
	}
}

func TestTagsForVersion_explicitWithoutV(t *testing.T) {
	got, err := tagsForVersion(nil, "example.com/ns/skill", "1.2.3", true)
	if err != nil {
		t.Fatalf("tagsForVersion() error: %v", err)
	}
	if len(got) != 2 || got[0] != "v1.2.3" || got[1] != "1.2.3" {
		t.Errorf("tagsForVersion() = %v, want [v1.2.3 1.2.3]", got)
	}
}

// ---- Resolve ----

func TestResolve_noSources(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	_, _, err := Resolve(context.Background(), []string{}, "ansible", "", false)
	if err == nil || !strings.Contains(err.Error(), "no sources configured") {
		t.Errorf("expected 'no sources configured', got: %v", err)
	}
}

func TestResolve_allSourcesUnreachable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// dead server: ListTags (empty version) fails → tried stays empty
	_, _, err := Resolve(context.Background(), []string{"127.0.0.1:1/ns"}, "ansible", "", false)
	if err == nil || !strings.Contains(err.Error(), "could not reach") {
		t.Errorf("expected 'could not reach', got: %v", err)
	}
}

func TestResolve_skillNotFound_triedAll(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// explicit version: tagsForVersion is pure (no network), returns ["v1.2.3","1.2.3"]
	// but ResolveDigest fails on dead server → tried is non-empty → "not found in any"
	_, _, err := Resolve(context.Background(), []string{"127.0.0.1:1/ns"}, "ansible", "v1.2.3", false)
	if err == nil || !strings.Contains(err.Error(), "not found in any configured source") {
		t.Errorf("expected 'not found in any configured source', got: %v", err)
	}
}

func TestResolve_success(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	wantDigest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	manifestBody := []byte(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","layers":[]}`)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"tags": []string{"v1.0.0"}})
			return
		}
		if strings.Contains(r.URL.Path, "/manifests/") {
			w.Header().Set("Content-Type", "application/vnd.oci.image.manifest.v1+json")
			w.Header().Set("Docker-Content-Digest", wantDigest)
			if r.Method == http.MethodGet {
				w.Header().Set("Content-Length", "0")
				w.Write(manifestBody)
			}
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	ref, dig, err := Resolve(context.Background(), []string{host + "/ns"}, "ansible", "", true)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if ref == "" {
		t.Error("Resolve() returned empty ref")
	}
	if dig != wantDigest {
		t.Errorf("Resolve() digest = %q, want %q", dig, wantDigest)
	}
}

// ---- tagsForVersion (empty version network paths) ----

func TestTagsForVersion_emptyVersion_noSemver(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"tags": []string{"latest", "stable"}})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	_, err := tagsForVersion(context.Background(), host+"/ns/ansible", "", true)
	if err == nil || !strings.Contains(err.Error(), "no semver tags") {
		t.Errorf("expected 'no semver tags' error, got: %v", err)
	}
}

func TestTagsForVersion_emptyVersion_tagsError(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// dead server: ListTags returns error
	_, err := tagsForVersion(context.Background(), "127.0.0.1:1/ns/ansible", "", false)
	if err == nil {
		t.Error("expected error from dead server, got nil")
	}
}
