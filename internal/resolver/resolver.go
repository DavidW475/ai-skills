package resolver

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/DavidW475/ai-skills/internal/registry"
)

// Resolve finds the best matching version of a skill across all configured sources.
//
// version == "": lists tags from each source and picks the highest semver tag.
// Returns an error if no semver tags are found.
//
// explicit version (e.g. "1.2.3" or "v1.2.3"): tries both ":v1.2.3" and
// ":1.2.3" so tags with or without the v-prefix work transparently.
func Resolve(ctx context.Context, sources []string, name, version string, plainHTTP bool) (string, string, error) {
	if len(sources) == 0 {
		return "", "", fmt.Errorf("no sources configured — run: ai-skills source add <registry>")
	}

	var tried []string
	for _, src := range sources {
		repoRef := strings.TrimRight(src, "/") + "/" + name

		tags, err := tagsForVersion(ctx, repoRef, version, plainHTTP)
		if err != nil {
			// Source unreachable for this skill — try next
			continue
		}

		for _, tag := range tags {
			ref := repoRef + ":" + tag
			digest, resolveErr := registry.ResolveDigest(ctx, ref, plainHTTP)
			if resolveErr == nil {
				return ref, digest, nil
			}
			tried = append(tried, ref)
		}
	}

	if len(tried) == 0 {
		return "", "", fmt.Errorf("skill %q not found — could not reach any configured source", name)
	}
	return "", "", fmt.Errorf("skill %q not found in any configured source\ntried:\n  %s",
		name, strings.Join(tried, "\n  "))
}

// tagsForVersion returns the ordered list of tags to attempt for a given version spec.
func tagsForVersion(ctx context.Context, repoRef, version string, plainHTTP bool) ([]string, error) {
	if version == "" {
		remoteTags, err := registry.ListTags(ctx, repoRef, plainHTTP)
		if err != nil {
			return nil, err
		}
		sv := highestSemver(remoteTags)
		if sv == "" {
			return nil, fmt.Errorf("no semver tags found in %s", repoRef)
		}
		return []string{sv}, nil
	}
	// Explicit version: try v-prefixed form first, then bare form.
	// Handles both "ansible@1.2.3" and "ansible@v1.2.3" regardless of how
	// the publisher tagged the image.
	stripped := strings.TrimPrefix(version, "v")
	return []string{"v" + stripped, stripped}, nil
}

// highestSemver returns the original tag string for the highest semver in the
// list, or "" if no valid semver tags are present.
func highestSemver(tags []string) string {
	var best semVer
	found := false
	for _, t := range tags {
		sv, ok := parseSemVer(t)
		if !ok {
			continue
		}
		if !found || compareSemVer(sv, best) > 0 {
			best = sv
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.raw
}

type semVer struct {
	major, minor, patch int
	pre                 string // pre-release label, e.g. "alpha.1"
	raw                 string // original tag as stored in the registry
}

// parseSemVer parses tags like "v1.2.3", "1.2.3", "v1.2.3-alpha.1".
// Returns (zero, false) for non-semver strings.
func parseSemVer(tag string) (semVer, bool) {
	s := strings.TrimPrefix(tag, "v")
	parts := strings.SplitN(s, "-", 2)
	var pre string
	if len(parts) == 2 {
		pre = parts[1]
	}
	nums := strings.Split(parts[0], ".")
	if len(nums) != 3 {
		return semVer{}, false
	}
	major, err1 := strconv.Atoi(nums[0])
	minor, err2 := strconv.Atoi(nums[1])
	patch, err3 := strconv.Atoi(nums[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return semVer{}, false
	}
	return semVer{major, minor, patch, pre, tag}, true
}

// compareSemVer returns >0 if a > b, 0 if equal, <0 if a < b.
// Stable releases (empty pre) sort higher than pre-releases.
func compareSemVer(a, b semVer) int {
	if d := a.major - b.major; d != 0 {
		return d
	}
	if d := a.minor - b.minor; d != 0 {
		return d
	}
	if d := a.patch - b.patch; d != 0 {
		return d
	}
	switch {
	case a.pre == "" && b.pre != "":
		return 1 // stable > pre-release
	case a.pre != "" && b.pre == "":
		return -1
	default:
		return strings.Compare(a.pre, b.pre)
	}
}

// LatestTag returns the highest semver tag from the given list, or "" if none found.
func LatestTag(tags []string) string { return highestSemver(tags) }

// IsNewer reports whether candidate is a strictly higher semver than current.
// Returns false if either string is not valid semver.
func IsNewer(candidate, current string) bool {
	a, ok1 := parseSemVer(candidate)
	b, ok2 := parseSemVer(current)
	if !ok1 || !ok2 {
		return false
	}
	return compareSemVer(a, b) > 0
}
