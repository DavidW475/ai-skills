package aiskills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/sources"
)

func newSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage skill sources (registries)",
		Long:  `source manages the list of OCI registry namespaces in ~/.ai-skills/sources.`,
	}
	cmd.AddCommand(newSourceAddCmd(), newSourceRemoveCmd(), newSourceListCmd())
	return cmd
}

func newSourceAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <registry>",
		Short: "Add a registry namespace to ~/.ai-skills/sources",
		Example: `  ai-skills source add ghcr.io/myorg/skills
  ai-skills source add registry.gitlab.com/david1904/pipelines/images`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sf, err := sources.Load()
			if err != nil {
				return err
			}
			registry := args[0]
			if !sf.Add(registry) {
				fmt.Fprintf(cmd.OutOrStdout(), "already in sources: %s\n", registry)
				return nil
			}
			if err := sources.Save(sf); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added source: %s\n", registry)
			return nil
		},
	}
	return cmd
}

func newSourceRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <registry>",
		Short: "Remove a registry namespace from ~/.ai-skills/sources",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sf, err := sources.Load()
			if err != nil {
				return err
			}
			registry := args[0]
			if !sf.Remove(registry) {
				fmt.Fprintf(cmd.OutOrStdout(), "not in sources: %s\n", registry)
				return nil
			}
			if err := sources.Save(sf); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed source: %s\n", registry)
			return nil
		},
	}
	return cmd
}

func newSourceListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured skill sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sf, err := sources.Load()
			if err != nil {
				return err
			}
			if len(sf.Sources) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No sources configured. Run: ai-skills source add <registry>")
				return nil
			}
			for _, s := range sf.Sources {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return nil
		},
	}
	return cmd
}
