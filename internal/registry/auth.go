package registry

import (
	"context"
	"fmt"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// newRepository creates a remote.Repository wired to Docker credential helpers.
// It transparently supports ghcr.io, ACR (azurecr.io), GitLab (registry.gitlab.com)
// and any other OCI-compatible registry that has been authenticated via `docker login`
// or a platform-specific login tool (gh auth login, az acr login, etc.).
func newRepository(ref string, plainHTTP bool) (*remote.Repository, error) {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %q: %w", ref, err)
	}

	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{
		AllowPlaintextPut: false,
	})
	if err != nil {
		return nil, fmt.Errorf("open credential store: %w (run 'ai-skills login %s' to authenticate)", err, registryFromRef(ref))
	}

	credFunc := credentials.Credential(store)

	repo.Client = &auth.Client{
		Credential: credFunc,
		Cache:      auth.DefaultCache,
	}
	repo.PlainHTTP = plainHTTP
	return repo, nil
}

// registryFromRef extracts the registry hostname from a fully-qualified reference.
func registryFromRef(ref string) string {
	repo, err := remote.NewRepository(ref)
	if err != nil {
		return ref
	}
	return repo.Reference.Registry
}

// Login saves credentials for a registry into the Docker credential store.
func Login(ctx context.Context, registry, username, password string) error {
	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{
		AllowPlaintextPut: true,
	})
	if err != nil {
		return fmt.Errorf("open credential store: %w", err)
	}
	return store.Put(ctx, registry, auth.Credential{
		Username: username,
		Password: password,
	})
}
