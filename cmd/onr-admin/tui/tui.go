package tui

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/cmd/onr-admin/store"
	"github.com/r9s-ai/open-next-router/internal/keystore"
	"github.com/r9s-ai/open-next-router/internal/models"
	"github.com/r9s-ai/open-next-router/pkg/dslconfig"
	"gopkg.in/yaml.v3"
)

type app struct {
	in           *bufio.Reader
	out          io.Writer
	cfgPath      string
	keysPath     string
	modelsPath   string
	providersDir string
	backup       bool
	masterKey    string

	keysDoc     *yaml.Node
	keysDirty   bool
	modelsDoc   *yaml.Node
	modelsDirty bool
}

func Run(cfgPath, keysPath, modelsPath string, backup bool, in io.Reader, out io.Writer) error {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, modelsPath = store.ResolveDataPaths(cfg, keysPath, modelsPath)
	secret := store.ResolveMasterKey(cfg)

	keysDoc, err := store.LoadOrInitKeysDoc(keysPath)
	if err != nil {
		return fmt.Errorf("load keys: %w", err)
	}
	modelsDoc, err := store.LoadOrInitModelsDoc(modelsPath)
	if err != nil {
		return fmt.Errorf("load models: %w", err)
	}

	t := &app{
		in:         bufio.NewReader(in),
		out:        out,
		cfgPath:    cfgPath,
		keysPath:   strings.TrimSpace(keysPath),
		modelsPath: strings.TrimSpace(modelsPath),
		backup:     backup,
		masterKey:  secret,
		keysDoc:    keysDoc,
		modelsDoc:  modelsDoc,
	}
	if cfg != nil {
		t.providersDir = strings.TrimSpace(cfg.Providers.Dir)
	}
	if strings.TrimSpace(t.providersDir) == "" {
		t.providersDir = "./config/providers"
	}
	return t.run()
}

func (a *app) run() error {
	for {
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, "+--------------------------------------------------+")
		fmt.Fprintln(a.out, "| ONR Admin TUI                                    |")
		fmt.Fprintln(a.out, "+--------------------------------------------------+")
		fmt.Fprintf(a.out, " config       : %s\n", strings.TrimSpace(a.cfgPath))
		fmt.Fprintf(a.out, " keys.yaml    : %s\n", strings.TrimSpace(a.keysPath))
		fmt.Fprintf(a.out, " models.yaml  : %s\n", strings.TrimSpace(a.modelsPath))
		fmt.Fprintf(a.out, " providers dir: %s\n", strings.TrimSpace(a.providersDir))
		fmt.Fprintf(a.out, " dirty        : keys=%v models=%v\n", a.keysDirty, a.modelsDirty)
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, " [k] keys.yaml 管理")
		fmt.Fprintln(a.out, " [m] models.yaml 管理")
		fmt.Fprintln(a.out, " [t] 生成 Token Key (onr:v1?)")
		fmt.Fprintln(a.out, " [v] 校验/诊断")
		fmt.Fprintln(a.out, " [s] 保存所有脏文件")
		fmt.Fprintln(a.out, " [q] 退出")

		choice, err := a.readMenuChoice("选择(k/m/t/v/s/q): ", []string{"k", "m", "t", "v", "s", "q"})
		if err != nil {
			return err
		}
		switch choice {
		case "k":
			if err := a.menuKeys(); err != nil {
				return err
			}
		case "m":
			if err := a.menuModels(); err != nil {
				return err
			}
		case "t":
			if err := a.menuGenKey(); err != nil {
				return err
			}
		case "v":
			if err := a.menuValidate(); err != nil {
				return err
			}
		case "s":
			if err := a.saveAll(); err != nil {
				return err
			}
		case "q":
			if a.keysDirty || a.modelsDirty {
				ok, err := a.confirm("有未保存的更改，确认退出？(y/N): ")
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
			}
			return nil
		}
	}
}

func (a *app) menuGenKey() error {
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== 生成 Token Key (onr:v1?) ==")
	accessKey, err := a.pickAccessKey()
	if err != nil {
		return err
	}

	p, err := a.readLine("provider p(可空): ")
	if err != nil {
		return err
	}
	m, err := a.readLine("model override m(可空): ")
	if err != nil {
		return err
	}
	uk, err := a.readLine("BYOK upstream key uk(可空): ")
	if err != nil {
		return err
	}

	vals := url.Values{}
	vals.Set("k64", base64.RawURLEncoding.EncodeToString([]byte(accessKey)))
	if strings.TrimSpace(p) != "" {
		vals.Set("p", strings.ToLower(strings.TrimSpace(p)))
	}
	if strings.TrimSpace(m) != "" {
		vals.Set("m", strings.TrimSpace(m))
	}
	if strings.TrimSpace(uk) != "" {
		vals.Set("uk", strings.TrimSpace(uk))
	}

	key := "onr:v1?" + vals.Encode()
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "生成结果：")
	fmt.Fprintln(a.out, key)
	return nil
}

func (a *app) pickAccessKey() (string, error) {
	ks, err := keystore.Load(a.keysPath)
	if err != nil {
		fmt.Fprintf(a.out, "提示：无法加载 keys.yaml 解析 access_keys（可能缺少 ONR_MASTER_KEY 以解密 ENC[...]）：%v\n", err)
	}
	aks := []keystore.AccessKey{}
	if ks != nil {
		aks = ks.AccessKeys()
	}

	fmt.Fprintln(a.out, "选择 access_key:")
	if len(aks) > 0 {
		for i, ak := range aks {
			name := strings.TrimSpace(ak.Name)
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			c := strings.TrimSpace(ak.Comment)
			if c != "" {
				c = " (" + c + ")"
			}
			fmt.Fprintf(a.out, "%d) %s%s\n", i+1, name, c)
		}
	}
	fmt.Fprintln(a.out, "m) 手动输入 access_key")

	s, err := a.readLine("选择(数字/m): ")
	if err != nil {
		return "", err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "m" || s == "" {
		def := strings.TrimSpace(a.masterKey)
		if def != "" {
			fmt.Fprintln(a.out, "提示：默认使用 auth.api_key 作为 access_key（仅用于演示；推荐在 keys.yaml 配置 access_keys）。")
		}
		in, err := a.readLine("access_key(留空=使用默认): ")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(in) != "" {
			def = strings.TrimSpace(in)
		}
		if def == "" {
			return "", errors.New("missing access_key")
		}
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > len(aks) {
		return "", errors.New("invalid selection")
	}
	return strings.TrimSpace(aks[n-1].Value), nil
}

func (a *app) menuValidate() error {
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== 校验/诊断 ==")
	if err := store.ValidateKeysDoc(a.keysDoc); err != nil {
		fmt.Fprintf(a.out, "keys.yaml(结构) 校验失败: %v\n", err)
	} else {
		fmt.Fprintln(a.out, "keys.yaml(结构) 校验: OK")
	}
	if _, err := keystore.Load(a.keysPath); err != nil {
		fmt.Fprintf(a.out, "keystore.Load(%s) 失败: %v\n", a.keysPath, err)
	} else {
		fmt.Fprintf(a.out, "keystore.Load(%s): OK\n", a.keysPath)
	}
	if err := store.ValidateModelsDoc(a.modelsDoc); err != nil {
		fmt.Fprintf(a.out, "models.yaml(结构) 校验失败: %v\n", err)
	} else {
		fmt.Fprintln(a.out, "models.yaml(结构) 校验: OK")
	}
	if _, err := models.Load(a.modelsPath); err != nil {
		fmt.Fprintf(a.out, "models.Load(%s) 失败: %v\n", a.modelsPath, err)
	} else {
		fmt.Fprintf(a.out, "models.Load(%s): OK\n", a.modelsPath)
	}
	if strings.TrimSpace(a.providersDir) != "" {
		if _, err := dslconfig.ValidateProvidersDir(a.providersDir); err != nil {
			fmt.Fprintf(a.out, "providers dir 校验失败 (%s): %v\n", a.providersDir, err)
		} else {
			fmt.Fprintf(a.out, "providers dir 校验: OK (%s)\n", a.providersDir)
		}
	}
	return nil
}

func (a *app) saveAll() error {
	if !a.keysDirty && !a.modelsDirty {
		fmt.Fprintln(a.out, "没有需要保存的更改。")
		return nil
	}
	if a.keysDirty {
		if err := store.ValidateKeysDoc(a.keysDoc); err != nil {
			return err
		}
		b, err := store.EncodeYAML(a.keysDoc)
		if err != nil {
			return err
		}
		if err := store.WriteAtomic(a.keysPath, b, a.backup); err != nil {
			return err
		}
		a.keysDirty = false
		fmt.Fprintln(a.out, "已保存 keys.yaml。")
	}
	if a.modelsDirty {
		if err := store.ValidateModelsDoc(a.modelsDoc); err != nil {
			return err
		}
		b, err := store.EncodeYAML(a.modelsDoc)
		if err != nil {
			return err
		}
		if err := store.WriteAtomic(a.modelsPath, b, a.backup); err != nil {
			return err
		}
		a.modelsDirty = false
		fmt.Fprintln(a.out, "已保存 models.yaml。")
	}
	return nil
}

func (a *app) readChoice(min, max int) (int, error) {
	for {
		s, err := a.readLine("选择: ")
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			continue
		}
		if n < min || n > max {
			continue
		}
		return n, nil
	}
}

func (a *app) readMenuChoice(prompt string, allows []string) (string, error) {
	set := make(map[string]struct{}, len(allows))
	for _, v := range allows {
		set[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	for {
		s, err := a.readLine(prompt)
		if err != nil {
			return "", err
		}
		s = strings.ToLower(strings.TrimSpace(s))
		if _, ok := set[s]; ok {
			return s, nil
		}
	}
}

func (a *app) readInt(prompt string) (int, error) {
	for {
		s, err := a.readLine(prompt)
		if err != nil {
			return 0, err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		return n, nil
	}
}

func (a *app) confirm(prompt string) (bool, error) {
	s, err := a.readLine(prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func (a *app) readLine(prompt string) (string, error) {
	fmt.Fprint(a.out, prompt)
	s, err := a.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(s, "\r\n"), nil
}
