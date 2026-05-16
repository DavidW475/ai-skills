package aiskills

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ai-skills",
	Short: "A package manager for private AI Skills",
	Long: `ai-skills manages AI Skill packages stored in OCI-compatible registries.

Supported registries: GitHub Container Registry (ghcr.io),
Azure Container Registry (azurecr.io), GitLab Container Registry,
and any other OCI-compliant registry.`,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(
		newInitCmd(),
		newPublishCmd(),
		newInstallCmd(),
		newUpdateCmd(),
		newUninstallCmd(),
		newListCmd(),
		newSourceCmd(),
		newLoginCmd(),
		newVersionsCmd(),
		newUICmd(),
		newSearchCmd(),
	)
}
