package registry

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"

	"github.com/DavidW475/ai-skills/internal/skill"
)

const (
	// MediaTypeSkillLayer is the OCI layer media type for an AI skill tar.gz archive.
	MediaTypeSkillLayer = "application/vnd.ai-skills.skill.content.v1.tar+gzip"
	// ArtifactType identifies the overall artifact in the OCI manifest.
	ArtifactType = "application/vnd.ai-skills.skill.v1"
)

// Push packages the skill at dir and pushes it to the OCI registry reference.
// ref must be a fully qualified reference: <registry>/<repository>:<tag>
// It returns the manifest digest string (e.g. "sha256:abc...") on success.
func Push(ctx context.Context, dir, ref string, plainHTTP bool) (string, error) {
	if _, err := skill.LoadManifest(dir); err != nil {
		return "", err
	}

	// Pack the skill directory into an in-memory tar.gz
	var buf bytes.Buffer
	if err := skill.Pack(dir, &buf); err != nil {
		return "", err
	}
	layerBytes := buf.Bytes()

	store := memory.New()

	// Push the layer blob
	layerDesc := ocispec.Descriptor{
		MediaType: MediaTypeSkillLayer,
		Digest:    digest.FromBytes(layerBytes),
		Size:      int64(len(layerBytes)),
	}
	if err := store.Push(ctx, layerDesc, bytes.NewReader(layerBytes)); err != nil {
		return "", fmt.Errorf("push: layer: %w", err)
	}

	// Build and push the OCI manifest.
	//
	// Use the standard OCI image config type as the config mediaType.
	// GitLab's registry validates config media types and rejects unknown vendor
	// types (e.g. "application/vnd.ai-skills.skill.v1"). Using the standard
	// image config type ensures compatibility with all OCI registries.
	// Skills are still identifiable by the layer mediaType (MediaTypeSkillLayer).
	//
	// The artifact type is preserved in a manifest annotation so it can be
	// inspected without pulling the layer.
	emptyConfig := []byte("{}")
	configDesc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageConfig,
		Digest:    digest.FromBytes(emptyConfig),
		Size:      int64(len(emptyConfig)),
	}
	if err := store.Push(ctx, configDesc, bytes.NewReader(emptyConfig)); err != nil {
		return "", fmt.Errorf("push: config: %w", err)
	}

	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_0, "", oras.PackManifestOptions{
		Layers:           []ocispec.Descriptor{layerDesc},
		ConfigDescriptor: &configDesc,
		ManifestAnnotations: map[string]string{
			"ai-skills.artifact.type": ArtifactType,
		},
	})
	if err != nil {
		return "", fmt.Errorf("push: pack manifest: %w", err)
	}

	// Copy from in-memory store to the remote registry
	repo, err := newRepository(ref, plainHTTP)
	if err != nil {
		return "", err
	}
	tag := extractTag(ref)
	if tag == "" {
		return "", fmt.Errorf("push: reference %q has no tag; use <repo>:<tag>", ref)
	}
	// Tag the manifest in the memory store so oras.Copy can resolve it by name.
	// memory.Store only resolves tagged references, not bare digest strings.
	if err := store.Tag(ctx, manifestDesc, tag); err != nil {
		return "", fmt.Errorf("push: tag manifest in store: %w", err)
	}
	if _, err := oras.Copy(ctx, store, tag, repo, tag, oras.DefaultCopyOptions); err != nil {
		return "", wrapAuthError(err, ref)
	}

	return manifestDesc.Digest.String(), nil
}

// extractTag returns the tag portion of a reference (the part after the last colon,
// unless it looks like a digest algorithm prefix).
func extractTag(ref string) string {
	// Handle digest references (ref@sha256:...)
	if atIdx := strings.LastIndex(ref, "@"); atIdx != -1 {
		return ref[atIdx+1:]
	}
	if colonIdx := strings.LastIndex(ref, ":"); colonIdx != -1 {
		candidate := ref[colonIdx+1:]
		// A path segment would contain '/', a digest would contain ':'
		if !strings.ContainsAny(candidate, "/:") {
			return candidate
		}
	}
	return ""
}
