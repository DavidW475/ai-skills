package aiskills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/installer"
)

func newUpdateCmd() *cobra.Command {
	var plainHTTP bool
	cmd := &cobra.Command{
		Use:   "update [name...]",
		Short: "Update installed skills to their latest version",
		Long: `update re-resolves each skill in ~/.ai-skills/installed.yaml against
the configured sources and re-downloads any skill whose remote digest differs.
Without arguments all installed skills are updated; you can pass one or more
skill names to update only those.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			opts := installer.Options{PlainHTTP: plainHTTP}

			if len(args) > 0 {
				for _, name := range args {
					r, err := installer.InstallOne(ctx, name, "", opts)
					if err != nil {
						return fmt.Errorf("update %s: %w", name, err)
					}
					printResult(cmd, r)
				}
				return nil
			}

			results, err := installer.Install(ctx, opts)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nothing to update (no skills installed yet)")
				return nil
			}
			for _, r := range results {
				printResult(cmd, r)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use plain HTTP (for local registries)")
	return cmd
}

func printResult(cmd *cobra.Command, r installer.Result) {
	if r.Skipped {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ %s  (up-to-date)\n", r.Name)
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  ↓ %s  %s → %s\n", r.Name, r.Ref, r.Path)
	}
}
