package resolver

import (
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
