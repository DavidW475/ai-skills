package aiskills

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/lockfile"
	"github.com/DavidW475/ai-skills/internal/registry"
	"github.com/DavidW475/ai-skills/internal/resolver"
	"github.com/DavidW475/ai-skills/internal/sources"
)

func newSearchCmd() *cobra.Command {
	var plainHTTP bool

	return &cobra.Command{
		Use:     "search",
		Aliases: []string{"available"},
		Short:   "List all skills available in configured sources",
		Long: `search queries the catalog of every configured source registry and
lists all available skill names together with their latest version.

Examples:
  ai-skills search
  ai-skills available`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			sf, err := sources.Load()
			if err != nil {
				return err
			}
			if len(sf.Sources) == 0 {
				return fmt.Errorf("no sources configured — run: ai-skills source add <registry>")
			}

			lf, err := lockfile.Load()
			if err != nil {
				return err
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tLATEST\tSOURCE\tSTATUS")

			found := false
			for _, src := range sf.Sources {
				skills, err := registry.ListSkills(ctx, src, plainHTTP)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %v\n", src, err)
					continue
				}
				for _, name := range skills {
					repoRef := src + "/" + name
					tags, _ := registry.ListTags(ctx, repoRef, plainHTTP)
					latest := resolver.LatestTag(tags)
					if latest == "" && len(tags) > 0 {
						latest = tags[len(tags)-1]
					}
					if latest == "" {
						latest = "(unknown)"
					}
					status := ""
					if lf.Find(name) != nil {
						status = "installed"
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, latest, src, status)
					found = true
				}
			}
			w.Flush()

			if !found {
				fmt.Fprintln(cmd.OutOrStdout(), "No skills found in any configured source.")
			}
			return nil
		},
	}
}
