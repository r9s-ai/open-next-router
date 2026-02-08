package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/tui"
)

func Run(args []string) error {
	if len(args) == 0 {
		printRootHelp(os.Stdout)
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printRootHelp(os.Stdout)
		return nil
	case "tui":
		return runTUI(args[1:])
	case "token":
		return runToken(args[1:])
	case "crypto":
		return runCrypto(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "balance":
		return runBalance(args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return runTUI(args)
		}
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootHelp(w io.Writer) {
	fmt.Fprintln(w, "onr-admin - ONR 管理命令")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  onr-admin <command> <subcommand> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "命令:")
	fmt.Fprintln(w, "  token create            生成 Token Key (onr:v1?)")
	fmt.Fprintln(w, "  token create phase      分阶段输出 Token 生成结果")
	fmt.Fprintln(w, "  crypto encrypt          将明文加密为 ENC[v1:aesgcm:...]")
	fmt.Fprintln(w, "  crypto encrypt-keys     一键加密 keys.yaml 中明文 value")
	fmt.Fprintln(w, "  crypto gen-master-key   生成随机 ONR_MASTER_KEY")
	fmt.Fprintln(w, "  validate all            校验 keys/models/providers")
	fmt.Fprintln(w, "  validate keys           校验 keys.yaml")
	fmt.Fprintln(w, "  validate models         校验 models.yaml")
	fmt.Fprintln(w, "  validate providers      校验 providers DSL 目录")
	fmt.Fprintln(w, "  balance get             按 providers DSL 查询上游余额")
	fmt.Fprintln(w, "  tui                     打开交互式管理界面")
}

func runTUI(args []string) error {
	var cfgPath string
	var keysPath string
	var modelsPath string
	var backup bool

	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfgPath, "config", "onr.yaml", "config yaml path")
	fs.StringVar(&cfgPath, "c", "onr.yaml", "config yaml path")
	fs.StringVar(&keysPath, "keys", "", "keys.yaml path (override config keys.file)")
	fs.StringVar(&modelsPath, "models", "", "models.yaml path (override config models.file)")
	fs.BoolVar(&backup, "backup", true, "backup yaml before saving")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return tui.Run(cfgPath, keysPath, modelsPath, backup, os.Stdin, os.Stdout)
}
