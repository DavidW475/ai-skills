package aiskills

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/registry"
	"github.com/DavidW475/ai-skills/internal/sources"
)

func newVersionsCmd() *cobra.Command {
	var plainHTTP bool

	cmd := &cobra.Command{
		Use:   "versions <name>",
		Short: "List all available versions for a skill",
		Long: `versions queries every configured source registry and prints all available
versions (OCI tags) for the given skill name.

Examples:
  ai-skills versions ansible
  ai-skills versions sql-helper`,
		Aliases: []string{"tags"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]

			sf, err := sources.Load()
			if err != nil {
				return err
			}
			if len(sf.Sources) == 0 {
				return fmt.Errorf("no sources configured — run: ai-skills source add <registry>")
			}

			found := false
			for _, src := range sf.Sources {
				repoRef := strings.TrimRight(src, "/") + "/" + name
				tags, err := registry.ListTags(ctx, repoRef, plainHTTP)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", src, err)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "# %s\n", repoRef)
				for _, t := range tags {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", t)
				}
				found = true
			}
			if !found {
				return fmt.Errorf("skill %q not found in any configured source", name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use plain HTTP (for local registries)")
	return cmd
}
