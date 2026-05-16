package aiskills

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/lockfile"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all locally installed skills",
		Long: `list prints all skills recorded in ~/.ai-skills/installed.yaml.

Examples:
  ai-skills list
  ai-skills ls`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			lf, err := lockfile.Load()
			if err != nil {
				return err
			}
			if len(lf.Skills) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no skills installed")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tPATH")
			for _, e := range lf.Skills {
				fmt.Fprintf(w, "%s\t%s\t%s\n", e.Name, e.Resolved, e.Installed)
			}
			return w.Flush()
		},
	}
}
