package aiskills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/registry"
)

func newPublishCmd() *cobra.Command {
	var plainHTTP bool
	cmd := &cobra.Command{
		Use:   "publish <dir> <ref>",
		Short: "Pack a skill and push it to an OCI registry",
		Long: `publish packages the skill directory <dir> as an OCI artifact and pushes it
to the registry reference <ref>.

Examples:
  ai-skills publish ./my-skill ghcr.io/myorg/my-skill:v1.0.0
  ai-skills publish ./my-skill azurecr.io/myteam/my-skill:v2.0.0
  ai-skills publish ./my-skill registry.gitlab.com/mygroup/my-skill:latest`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, ref := args[0], args[1]
			ctx := cmd.Context()
			fmt.Fprintf(cmd.OutOrStdout(), "Publishing %s → %s\n", dir, ref)
			digest, err := registry.Push(ctx, dir, ref, plainHTTP)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Published %s\n  digest: %s\n", ref, digest)
			return nil
		},
	}
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use plain HTTP instead of HTTPS (for local registries)")
	return cmd
}
