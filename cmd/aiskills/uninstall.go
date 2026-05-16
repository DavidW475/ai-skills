package aiskills

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/lockfile"
)

func newUninstallCmd() *cobra.Command {
	var keepFiles bool

	cmd := &cobra.Command{
		Use:     "uninstall <name> [name...]",
		Aliases: []string{"remove", "rm"},
		Short:   "Uninstall one or more skills",
		Long: `uninstall removes the skill directory from the local filesystem and
removes the entry from ~/.ai-skills/installed.yaml.

Examples:
  ai-skills uninstall ansible
  ai-skills uninstall ansible sql-helper
  ai-skills uninstall --keep-files ansible`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(cmd, args, keepFiles)
		},
	}
	cmd.Flags().BoolVar(&keepFiles, "keep-files", false, "Remove from index but keep files on disk")
	return cmd
}

func runUninstall(cmd *cobra.Command, args []string, keepFiles bool) error {
	lf, err := lockfile.Load()
	if err != nil {
		return err
	}

	var errs []error
	for _, name := range args {
		if err := uninstallOne(cmd, lf, name, keepFiles); err != nil {
			errs = append(errs, err)
		}
	}

	if saveErr := lockfile.Save(lf); saveErr != nil {
		return saveErr
	}

	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", e)
		}
		return fmt.Errorf("%d skill(s) could not be uninstalled", len(errs))
	}
	return nil
}

func uninstallOne(cmd *cobra.Command, lf *lockfile.LockFile, name string, keepFiles bool) error {
	entry := lf.Find(name)
	if entry == nil {
		return fmt.Errorf("%s: not installed", name)
	}

	if !keepFiles && entry.Installed != "" {
		if rmErr := os.RemoveAll(entry.Installed); rmErr != nil {
			return fmt.Errorf("%s: remove files: %w", name, rmErr)
		}
	}

	lf.Remove(name)
	fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", name)
	return nil
}
