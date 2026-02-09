package cli

import (
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/tui"
	"github.com/spf13/cobra"
)

func Run(args []string) error {
	root := newRootCmd()
	if len(args) > 0 && strings.HasPrefix(args[0], "-") && args[0] != "-h" && args[0] != "--help" {
		// 兼容旧行为：直接传 flag 时默认进入 tui。
		args = append([]string{"tui"}, args...)
	}
	root.SetArgs(args)
	return root.Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "onr-admin",
		Short:         "ONR 管理命令",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.AddCommand(
		newTokenCmd(),
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
		backup:  true,
		stdin:   os.Stdin,
		stdout:  os.Stdout,
	}
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "打开交互式管理界面",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIWithOptions(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.cfgPath, "config", "c", "onr.yaml", "config yaml path")
	fs.StringVar(&opts.keysPath, "keys", "", "keys.yaml path (override config keys.file)")
	fs.StringVar(&opts.modelsPath, "models", "", "models.yaml path (override config models.file)")
	fs.BoolVar(&opts.backup, "backup", true, "backup yaml before saving")
	return cmd
}

type tuiOptions struct {
	cfgPath    string
	keysPath   string
	modelsPath string
	backup     bool
	stdin      *os.File
	stdout     *os.File
}

func runTUIWithOptions(opts tuiOptions) error {
	return tui.Run(opts.cfgPath, opts.keysPath, opts.modelsPath, opts.backup, opts.stdin, opts.stdout)
}
