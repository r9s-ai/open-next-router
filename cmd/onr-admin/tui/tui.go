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
)

type app struct {
	in           *bufio.Reader
	out          io.Writer
	cfgPath      string
	keysPath     string
	modelsPath   string
	providersDir string
	masterKey    string
}

func Run(cfgPath, keysPath, modelsPath string, in io.Reader, out io.Writer) error {
	cfg, _ := store.LoadConfigIfExists(strings.TrimSpace(cfgPath))
	keysPath, modelsPath = store.ResolveDataPaths(cfg, keysPath, modelsPath)
	secret := store.ResolveMasterKey(cfg)

	t := &app{
		in:         bufio.NewReader(in),
		out:        out,
		cfgPath:    cfgPath,
		keysPath:   strings.TrimSpace(keysPath),
		modelsPath: strings.TrimSpace(modelsPath),
		masterKey:  secret,
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
		fmt.Fprintf(a.out, " config        : %s\n", strings.TrimSpace(a.cfgPath))
		fmt.Fprintf(a.out, " keys.yaml     : %s\n", strings.TrimSpace(a.keysPath))
		fmt.Fprintf(a.out, " models.yaml   : %s\n", strings.TrimSpace(a.modelsPath))
		fmt.Fprintf(a.out, " providers dir : %s\n", strings.TrimSpace(a.providersDir))
		fmt.Fprintln(a.out, "")
		fmt.Fprintln(a.out, " [t] Generate Token Key (onr:v1?)")
		fmt.Fprintln(a.out, " [v] Validate / Diagnose")
		fmt.Fprintln(a.out, " [q] Quit")

		choice, err := a.readMenuChoice("Select (t/v/q): ", []string{"t", "v", "q"})
		if err != nil {
			return err
		}
		switch choice {
		case "t":
			if err := a.menuGenKey(); err != nil {
				return err
			}
		case "v":
			if err := a.menuValidate(); err != nil {
				return err
			}
		case "q":
			return nil
		}
	}
}

func (a *app) menuGenKey() error {
	fmt.Fprintln(a.out, "")
	fmt.Fprintln(a.out, "== Generate Token Key (onr:v1?) ==")
	accessKey, err := a.pickAccessKey()
	if err != nil {
		return err
	}

	p, err := a.readLine("provider (optional, maps to p=): ")
	if err != nil {
		return err
	}
	m, err := a.readLine("model override (optional, maps to m=): ")
	if err != nil {
		return err
	}
	uk, err := a.readLine("BYOK upstream key (optional, maps to uk=): ")
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
	fmt.Fprintln(a.out, "Result:")
	fmt.Fprintln(a.out, key)
	return nil
}

func (a *app) pickAccessKey() (string, error) {
	ks, err := keystore.Load(a.keysPath)
	if err != nil {
		fmt.Fprintf(a.out, "Note: failed to load keys.yaml for access_keys (maybe missing ONR_MASTER_KEY to decrypt ENC[...]): %v\n", err)
	}
	aks := []keystore.AccessKey{}
	if ks != nil {
		aks = ks.AccessKeys()
	}

	fmt.Fprintln(a.out, "Select an access key:")
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
	fmt.Fprintln(a.out, "m) Enter access_key manually")

	s, err := a.readLine("Select (number/m): ")
	if err != nil {
		return "", err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "m" || s == "" {
		def := strings.TrimSpace(a.masterKey)
		if def != "" {
			fmt.Fprintln(a.out, "Note: defaulting to auth.api_key as access_key (for convenience); prefer configuring access_keys in keys.yaml.")
		}
		in, err := a.readLine("access_key (empty = use default): ")
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
	fmt.Fprintln(a.out, "== Validate / Diagnose ==")
	if _, err := keystore.Load(a.keysPath); err != nil {
		fmt.Fprintf(a.out, "keys.yaml load failed (%s): %v\n", a.keysPath, err)
	} else {
		fmt.Fprintf(a.out, "keys.yaml load: OK (%s)\n", a.keysPath)
	}
	if _, err := models.Load(a.modelsPath); err != nil {
		fmt.Fprintf(a.out, "models.yaml load failed (%s): %v\n", a.modelsPath, err)
	} else {
		fmt.Fprintf(a.out, "models.yaml load: OK (%s)\n", a.modelsPath)
	}
	if strings.TrimSpace(a.providersDir) != "" {
		if _, err := dslconfig.ValidateProvidersDir(a.providersDir); err != nil {
			fmt.Fprintf(a.out, "providers dir validate failed (%s): %v\n", a.providersDir, err)
		} else {
			fmt.Fprintf(a.out, "providers dir validate: OK (%s)\n", a.providersDir)
		}
	}
	return nil
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
