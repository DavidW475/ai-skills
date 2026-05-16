package aiskills

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/installer"
)

func newInstallCmd() *cobra.Command {
	var (
		plainHTTP bool
		force     bool
	)
	cmd := &cobra.Command{
		Use:   "install <name>[@version]",
		Short: "Install a skill globally from configured sources",
		Long: `install resolves the given skill name against the registries listed
in ~/.ai-skills/sources, downloads it, and installs it into the global skills
directory (default ~/.agent/skills/<name>/, configurable via ~/.ai-skills/config.yaml).

The skill is recorded in ~/.ai-skills/installed.yaml.

Examples:
  ai-skills install ansible
  ai-skills install ansible@v0.1.0
  ai-skills install sql-helper`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name, version := parseNameVersion(args[0])
			r, err := installer.InstallOne(ctx, name, version, installer.Options{
				PlainHTTP: plainHTTP,
				Force:     force,
			})
			if err != nil {
				return err
			}
			printResult(cmd, r)
			return nil
		},
	}
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use plain HTTP (for local registries)")
	cmd.Flags().BoolVar(&force, "force", false, "Re-download even if digest matches installed index")
	return cmd
}

// parseNameVersion splits "ansible@v0.1.0" into ("ansible", "v0.1.0").
// If no @ is present, version is empty — the resolver will pick the highest semver tag.
func parseNameVersion(arg string) (name, version string) {
	if i := strings.LastIndex(arg, "@"); i != -1 {
		return arg[:i], arg[i+1:]
	}
	return arg, ""
}
