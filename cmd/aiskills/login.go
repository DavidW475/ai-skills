package aiskills

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/DavidW475/ai-skills/internal/registry"
)

func newLoginCmd() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "login <registry>",
		Short: "Authenticate with an OCI registry",
		Long: `login stores credentials for the given registry in the Docker credential store.
The stored credentials are used automatically by install and publish.

Examples:
  ai-skills login ghcr.io
  ai-skills login myregistry.azurecr.io
  ai-skills login registry.gitlab.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg := args[0]
			ctx := context.Background()

			if username == "" {
				username = prompt("Username: ")
			}
			password, err := readPassword("Password: ")
			if err != nil {
				return fmt.Errorf("reading password: %w", err)
			}

			if err := registry.Login(ctx, reg, username, password); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to %s as %s\n", reg, username)
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Registry username")
	return cmd
}

func prompt(label string) string {
	fmt.Fprint(os.Stderr, label)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n")
}

func readPassword(label string) (string, error) {
	fmt.Fprint(os.Stderr, label)
	// Use terminal raw mode when stdin is a TTY so the password is not echoed.
	if term.IsTerminal(int(syscall.Stdin)) {
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr) // newline after hidden input
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	// Fallback for non-TTY (e.g. CI piped input)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimRight(line, "\r\n"), nil
}
