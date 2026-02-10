package cli

import (
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-admin/tui"
	"github.com/spf13/cobra"
)

func Run(args []string) error {
	root := newRootCmd()
	if len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-h" && args[0] != "--help" {
		// Backward compatibility: if only flags are provided, default to `tui`.
		args = append([]string{"tui"}, args...)
	}
	root.SetArgs(args)
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "onr-admin",
		Short:         "ONR admin CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newTokenCmd(),
		newOAuthCmd(),
		newCryptoCmd(),
		newValidateCmd(),
		newBalanceCmd(),
		newPricingCmd(),
		newTUICmd(),
	)
	return cmd
}

func newTUICmd() *cobra.Command {
	opts := tuiOptions{
		cfgPath: "onr.yaml",
		stdin:   os.Stdin,
		stdout:  os.Stdout,
	}
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Open interactive TUI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.cfgPath, "config", "c", "onr.yaml", "config yaml path")
	return cmd
}

type tuiOptions struct {
	cfgPath string
	stdin   *os.File
	stdout  *os.File
}

func runTUIWithOptions(opts tuiOptions) error {
	return tui.Run(opts.cfgPath, opts.stdin, opts.stdout)
}
