package aiskills

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/skill"
)

func newInitCmd() *cobra.Command {
	var version string
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Scaffold a new skill directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir := filepath.Join(".", name)
			if err := skill.Scaffold(dir, name, version); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created skill %q in %s\n", name, dir)
			fmt.Fprintf(cmd.OutOrStdout(), "  Edit %s/%s to describe your skill.\n", dir, skill.SkillFile)
			fmt.Fprintf(cmd.OutOrStdout(), "  Edit %s/%s to update metadata.\n", dir, skill.ManifestFile)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "v0.0.1", "Initial version for the skill")
	return cmd
}
