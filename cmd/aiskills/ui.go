package aiskills

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/DavidW475/ai-skills/internal/ui"
)

func newUICmd() *cobra.Command {
	var (
		addr      string
		plainHTTP bool
	)
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Start a local web UI",
		Long: `ui starts a local HTTP server and opens a web dashboard for
managing installed skills and sources.

Press Ctrl+C to stop the server.

Examples:
  ai-skills ui
  ai-skills ui --addr localhost:8080`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer cancel()

			url, err := ui.Listen(ctx, addr, plainHTTP)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "UI running at %s\nPress Ctrl+C to stop.\n", url)
			<-ctx.Done()
			fmt.Fprintln(cmd.OutOrStdout(), "\nstopped.")
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "localhost:8080", "Address to listen on")
	cmd.Flags().BoolVar(&plainHTTP, "plain-http", false, "Use plain HTTP for registry operations")
	return cmd
}
