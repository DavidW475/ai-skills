package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// wrapAuthError detects HTTP 401 responses and adds a login hint to the error.
func wrapAuthError(err error, ref string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
		return fmt.Errorf("%w\n\nhint: run 'ai-skills login %s' to authenticate", err, registryFromRef(ref))
	}
	return err
}

// Pull downloads the skill artifact at ref and returns the raw tar.gz bytes and
// the manifest digest string. The caller is responsible for unpacking.
func Pull(ctx context.Context, ref string, plainHTTP bool) ([]byte, string, error) {
	repo, err := newRepository(ref, plainHTTP)
	if err != nil {
		return nil, "", err
	}

	tag := extractTag(ref)
	if tag == "" {
		return nil, "", fmt.Errorf("pull: reference %q has no tag", ref)
	}

	store := memory.New()
	manifestDesc, err := oras.Copy(ctx, repo, tag, store, "", oras.DefaultCopyOptions)
	if err != nil {
		return nil, "", wrapAuthError(err, ref)
	}

	// Fetch and parse the manifest to locate the skill layer
	manifestRC, err := store.Fetch(ctx, manifestDesc)
	if err != nil {
		return nil, "", fmt.Errorf("pull: fetch manifest: %w", err)
	}
	defer manifestRC.Close()

	manifestBytes, err := io.ReadAll(manifestRC)
	if err != nil {
		return nil, "", fmt.Errorf("pull: read manifest: %w", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, "", fmt.Errorf("pull: parse manifest: %w", err)
	}

	var layerDesc *ocispec.Descriptor
	for i := range manifest.Layers {
		if manifest.Layers[i].MediaType == MediaTypeSkillLayer {
			layerDesc = &manifest.Layers[i]
			break
		}
	}
	if layerDesc == nil {
		return nil, "", fmt.Errorf("pull: no layer with media type %q found in manifest", MediaTypeSkillLayer)
	}

	layerRC, err := store.Fetch(ctx, *layerDesc)
	if err != nil {
		return nil, "", fmt.Errorf("pull: fetch layer: %w", err)
	}
	defer layerRC.Close()

	// Limit to 100 MB as a safety guard against oversized layers
	layerBytes, err := io.ReadAll(io.LimitReader(layerRC, 100<<20))
	if err != nil {
		return nil, "", fmt.Errorf("pull: read layer: %w", err)
	}

	return layerBytes, manifestDesc.Digest.String(), nil
}

// ResolveDigest resolves the manifest digest for a given ref without downloading
// the full layer. Useful for checking if an already-installed skill is up to date.
func ResolveDigest(ctx context.Context, ref string, plainHTTP bool) (string, error) {
	repo, err := newRepository(ref, plainHTTP)
	if err != nil {
		return "", err
	}
	tag := extractTag(ref)
	if tag == "" {
		return "", fmt.Errorf("resolve: reference %q has no tag", ref)
	}
	desc, err := repo.Resolve(ctx, tag)
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", ref, err)
	}
	return desc.Digest.String(), nil
}

// ListTags returns all tags available in the given repository.
// ref should be a repository path without a tag or digest
// (e.g. "ghcr.io/myorg/skills/ansible").
func ListTags(ctx context.Context, ref string, plainHTTP bool) ([]string, error) {
	repo, err := newRepository(ref, plainHTTP)
	if err != nil {
		return nil, err
	}
	var tags []string
	if err := repo.Tags(ctx, "", func(t []string) error {
		tags = append(tags, t...)
		return nil
	}); err != nil {
		return nil, wrapAuthError(err, ref)
	}
	return tags, nil
}

// ListSkills returns the names of all repositories found under the given source
// namespace. source has the form "host/namespace" (e.g.
// "registry.gitlab.com/david1904/skills"). The OCI catalog API is tried first;
// if that fails with an auth error and the host looks like a GitLab registry
// (e.g. registry.gitlab.com or registry.example.com), the GitLab REST API is
// tried as a fallback using the stored Docker credentials as a private token.
func ListSkills(ctx context.Context, source string, plainHTTP bool) ([]string, error) {
	parts := strings.SplitN(source, "/", 2)
	host := parts[0]
	namespace := ""
	if len(parts) == 2 {
		namespace = parts[1]
	}

	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{AllowPlaintextPut: false})
	if err != nil {
		return nil, fmt.Errorf("open credential store: %w", err)
	}

	reg, err := remote.NewRegistry(host)
	if err != nil {
		return nil, fmt.Errorf("invalid registry %q: %w", host, err)
	}
	reg.PlainHTTP = plainHTTP
	reg.Client = &auth.Client{
		Credential: credentials.Credential(credStore),
		Cache:      auth.DefaultCache,
	}

	catalogCtx := auth.WithScopes(ctx, "registry:catalog:*")
	skills, catalogErr := listSkillsViaCatalog(catalogCtx, reg, namespace)
	if catalogErr == nil {
		return skills, nil
	}

	return handleCatalogError(ctx, host, namespace, credStore, plainHTTP, catalogErr)
}

func listSkillsViaCatalog(ctx context.Context, reg *remote.Registry, namespace string) ([]string, error) {
	prefix := namespace + "/"
	var skills []string
	err := reg.Repositories(ctx, "", func(repos []string) error {
		for _, repo := range repos {
			if namespace == "" || strings.HasPrefix(repo, prefix) {
				name := strings.TrimPrefix(repo, prefix)
				if name != "" {
					skills = append(skills, name)
				}
			}
		}
		return nil
	})
	return skills, err
}

func handleCatalogError(ctx context.Context, host, namespace string, credStore credentials.Store, plainHTTP bool, catalogErr error) ([]string, error) {
	// If the catalog call failed due to auth/permission, try GitLab REST API.
	// GitLab restricts /v2/_catalog to admins; the projects API works for
	// regular users who have access to the project.
	msg := catalogErr.Error()
	isAuthErr := strings.Contains(msg, "401") || strings.Contains(msg, "nauthorized") ||
		strings.Contains(msg, "403") || strings.Contains(msg, "orbidden")
	hasRegistryPrefix := strings.HasPrefix(host, "registry.")

	if isAuthErr && hasRegistryPrefix && namespace != "" {
		result, gitlabErr := listSkillsViaGitLabAPI(ctx, host, namespace, credStore, plainHTTP)
		if gitlabErr == nil {
			return result, nil
		}
		// Surface the GitLab API error; it is more actionable than the catalog error.
		return nil, gitlabErr
	}

	if isAuthErr {
		return nil, fmt.Errorf(
			"catalog listing not available for %s: the registry requires admin access for /v2/_catalog\n"+
				"You can still install skills by name: ai-skills install <name>",
			host,
		)
	}
	return nil, catalogErr
}

type repoItem struct {
	Path string `json:"path"`
}

// listSkillsViaGitLabAPI lists container registry repositories for a project
// using the GitLab REST API. It derives the GitLab API host by stripping the
// leading "registry." from the registry hostname (e.g. registry.gitlab.com →
// gitlab.com), then calls GET /api/v4/projects/{namespace}/registry/repositories.
//
// The stored Docker password (typically a PAT or OAuth token) is tried first as
// a PRIVATE-TOKEN header, then as Authorization: Bearer.
func listSkillsViaGitLabAPI(ctx context.Context, regHost, namespace string, credStore credentials.Store, plainHTTP bool) ([]string, error) {
	apiHost := strings.TrimPrefix(regHost, "registry.")
	scheme := "https"
	if plainHTTP {
		scheme = "http"
	}

	// URL-encode the namespace: slashes become %2F for GitLab project path.
	encodedNS := strings.ReplaceAll(namespace, "/", "%2F")
	baseURL := fmt.Sprintf("%s://%s/api/v4/projects/%s/registry/repositories", scheme, apiHost, encodedNS)

	cred, err := credStore.Get(ctx, regHost)
	if err != nil || cred.Password == "" {
		return nil, fmt.Errorf("no credentials found for %s — run: ai-skills login %s", regHost, regHost)
	}

	// Try PRIVATE-TOKEN first (Personal Access Token); fall back to Bearer (OAuth).
	for _, authMethod := range []struct{ header, value string }{
		{"PRIVATE-TOKEN", cred.Password},
		{"Authorization", "Bearer " + cred.Password},
	} {
		allItems, err := collectAllPages(ctx, baseURL, authMethod.header, authMethod.value)
		if err != nil {
			continue // try next auth method
		}
		return extractSkillNames(allItems, namespace), nil
	}

	return nil, fmt.Errorf(
		"cannot list skills in %s/%s: GitLab API authentication failed\n"+
			"Make sure the token used with 'ai-skills login %s' has at least the 'read_registry' or 'api' scope",
		regHost, namespace, regHost,
	)
}

func collectAllPages(ctx context.Context, baseURL, tokenHeader, tokenValue string) ([]repoItem, error) {
	var allItems []repoItem
	nextURL := baseURL + "?per_page=100"
	for nextURL != "" {
		items, next, err := fetchGitLabPage(ctx, nextURL, baseURL, tokenHeader, tokenValue)
		if err != nil {
			return nil, err
		}
		allItems = append(allItems, items...)
		nextURL = next
	}
	return allItems, nil
}

func fetchGitLabPage(ctx context.Context, url, baseURL, tokenHeader, tokenValue string) ([]repoItem, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set(tokenHeader, tokenValue)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, "", fmt.Errorf("GitLab API returned %d for %s", resp.StatusCode, url)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var items []repoItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, "", fmt.Errorf("GitLab API response parse error: %w", err)
	}

	nextPage := resp.Header.Get("X-Next-Page")
	var nextURL string
	if nextPage != "" {
		nextURL = baseURL + "?per_page=100&page=" + nextPage
	}
	return items, nextURL, nil
}

func extractSkillNames(items []repoItem, namespace string) []string {
	prefix := namespace + "/"
	var skills []string
	for _, item := range items {
		name := strings.TrimPrefix(item.Path, prefix)
		if name != item.Path && name != "" {
			skills = append(skills, name)
		}
	}
	return skills
}
