package cli

import (
	"fmt"

	"github.com/r9s-ai/open-next-router/internal/version"
	"github.com/spf13/cobra"
)

// newVersionCmd returns a non-nil version command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.Get())
			return err
		},
	}
}
