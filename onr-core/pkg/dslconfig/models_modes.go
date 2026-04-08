package dslconfig

import (
	"fmt"
	"sort"
	"strings"
)

type modelsModeRegistry map[string]ModelsQueryConfig

func parseGlobalModelsModes(path string, content string) (modelsModeRegistry, error) {
	s := newScanner(path, content)
	modes := modelsModeRegistry{}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return modes, nil
		case tokIdent:
			switch tok.text {
			case "models_mode":
				name, cfg, err := parseModelsModeBlock(s)
				if err != nil {
					return nil, err
				}
				if _, exists := modes[name]; exists {
					return nil, fmt.Errorf("duplicate models_mode %q in %q", name, path)
				}
				modes[name] = normalizeModelsQueryConfig(cfg)
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return nil, err
				}
			}
		}
	}
}

func parseModelsModeBlock(s *scanner) (string, ModelsQueryConfig, error) {
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
	default:
		return "", ModelsQueryConfig{}, s.errAt(nameTok, "models_mode expects mode name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	name = normalizeUsageModeName(name)
	if name == "" {
		return "", ModelsQueryConfig{}, s.errAt(nameTok, "models_mode name is empty")
	}
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return "", ModelsQueryConfig{}, s.errAt(lb, "expected '{' after models_mode name")
	}

	var cfg ModelsQueryConfig
	var hdr PhaseHeaders
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return "", ModelsQueryConfig{}, s.errAt(tok, "unexpected EOF in models_mode block")
		case tokRBrace:
			if len(hdr.Request) > 0 {
				cfg.Headers = append(cfg.Headers, hdr.Request...)
			}
			return name, normalizeModelsQueryConfig(cfg), nil
		case tokIdent:
			switch tok.text {
			case "models_mode":
				mode, err := parseModeArgStmt(s, "models_mode")
				if err != nil {
					return "", ModelsQueryConfig{}, err
				}
				cfg.Mode = strings.TrimSpace(mode)
			case "method", "path", "id_regex", "id_allow_regex":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return "", ModelsQueryConfig{}, err
				}
				switch tok.text {
				case "method":
					cfg.Method = v
				case "path":
					cfg.Path = v
				case "id_regex":
					cfg.IDRegex = v
				case "id_allow_regex":
					cfg.IDAllowRegex = v
				}
			case "id_path":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return "", ModelsQueryConfig{}, err
				}
				v = strings.TrimSpace(v)
				if v != "" {
					cfg.IDPaths = append(cfg.IDPaths, v)
				}
			case "set_header":
				if err := parseSetHeaderStmt(s, &hdr); err != nil {
					return "", ModelsQueryConfig{}, err
				}
			case "del_header":
				if err := parseDelHeaderStmt(s, &hdr); err != nil {
					return "", ModelsQueryConfig{}, err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return "", ModelsQueryConfig{}, err
				}
			}
		}
	}
}

func resolveModelsModeRegistry(pathByMode map[string]string, raw modelsModeRegistry) (modelsModeRegistry, error) {
	merged := modelsModeRegistry{}
	paths := map[string]string{}
	for name, cfg := range raw {
		merged[name] = cfg
	}
	for name, path := range pathByMode {
		paths[name] = path
	}
	for name, cfg := range merged {
		if strings.TrimSpace(cfg.Mode) == "" {
			if builtin := builtinModelsPresetName(name); builtin != "" {
				cfg.Mode = builtin
			} else if hasAnyModelsQueryRule(cfg) {
				cfg.Mode = modelsModeCustom
			}
			merged[name] = normalizeModelsQueryConfig(cfg)
		}
	}
	resolved := modelsModeRegistry{}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg, err := resolveModelsQueryConfig(paths[name], "", fmt.Sprintf("models_mode %q", name), merged[name], merged, nil)
		if err != nil {
			return nil, err
		}
		if err := validateResolvedModelsQueryConfig(paths[name], "", fmt.Sprintf("models_mode %q", name), cfg); err != nil {
			return nil, err
		}
		resolved[name] = normalizeModelsQueryConfig(cfg)
	}
	return resolved, nil
}

func resolveModelsQueryConfig(path, providerName, scope string, cfg ModelsQueryConfig, registry modelsModeRegistry, stack []string) (ModelsQueryConfig, error) {
	mode := normalizeUsageMode(cfg.Mode)
	switch mode {
	case "":
		return inferImplicitCustomModelsQueryConfig(cfg), nil
	case modelsModeCustom:
		return normalizeModelsQueryConfig(cfg), nil
	}
	if builtinModelsPresetName(mode) != "" && len(stack) > 0 && stack[len(stack)-1] == mode {
		return normalizeModelsQueryConfig(cfg), nil
	}
	if baseCfg, ok := registry[mode]; ok {
		for _, item := range stack {
			if item == mode {
				return ModelsQueryConfig{}, fmt.Errorf("%s in %q: recursive models_mode reference %q", scope, path, mode)
			}
		}
		base, err := resolveModelsQueryConfig(path, providerName, fmt.Sprintf("models_mode %q", mode), baseCfg, registry, append(stack, mode))
		if err != nil {
			return ModelsQueryConfig{}, err
		}
		if builtinModelsPresetName(mode) != "" {
			base.Mode = mode
		}
		override := cfg
		override.Mode = ""
		return mergeModelsQueryConfig(base, override), nil
	}
	if (providerName == "" || len(stack) > 0) && builtinModelsPresetName(mode) != "" {
		return normalizeModelsQueryConfig(cfg), nil
	}
	return ModelsQueryConfig{}, validationIssue(
		fmt.Errorf("provider %q in %q: %s unsupported models_mode %q", providerName, path, scope, cfg.Mode),
		scope,
		"models_mode",
	)
}
