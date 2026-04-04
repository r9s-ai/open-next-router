package dslconfig

import (
	"fmt"
	"sort"
	"strings"
)

type finishReasonModeRegistry map[string]FinishReasonExtractConfig

func parseGlobalFinishReasonModes(path string, content string) (finishReasonModeRegistry, error) {
	s := newScanner(path, content)
	modes := finishReasonModeRegistry{}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return modes, nil
		case tokIdent:
			switch tok.text {
			case "finish_reason_mode":
				name, cfg, err := parseFinishReasonModeBlock(s)
				if err != nil {
					return nil, err
				}
				if _, exists := modes[name]; exists {
					return nil, fmt.Errorf("duplicate finish_reason_mode %q in %q", name, path)
				}
				modes[name] = normalizeFinishReasonExtractConfig(cfg)
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return nil, err
				}
			}
		}
	}
}

func parseFinishReasonModeBlock(s *scanner) (string, FinishReasonExtractConfig, error) {
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
	default:
		return "", FinishReasonExtractConfig{}, s.errAt(nameTok, "finish_reason_mode expects mode name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	name = normalizeUsageModeName(name)
	if name == "" {
		return "", FinishReasonExtractConfig{}, s.errAt(nameTok, "finish_reason_mode name is empty")
	}
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return "", FinishReasonExtractConfig{}, s.errAt(lb, "expected '{' after finish_reason_mode name")
	}

	var cfg FinishReasonExtractConfig
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return "", FinishReasonExtractConfig{}, s.errAt(tok, "unexpected EOF in finish_reason_mode block")
		case tokRBrace:
			return name, normalizeFinishReasonExtractConfig(cfg), nil
		case tokIdent:
			switch tok.text {
			case "finish_reason_extract":
				if err := parseFinishReasonExtractStmt(s, &cfg); err != nil {
					return "", FinishReasonExtractConfig{}, err
				}
			case "finish_reason_path":
				if err := parseFinishReasonPathStmt(s, &cfg); err != nil {
					return "", FinishReasonExtractConfig{}, err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return "", FinishReasonExtractConfig{}, err
				}
			}
		}
	}
}

func normalizeFinishReasonMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

func normalizeFinishReasonExtractConfig(cfg FinishReasonExtractConfig) FinishReasonExtractConfig {
	if len(cfg.paths) > 0 {
		cfg.paths = append([]finishReasonPathConfig(nil), cfg.paths...)
		cfg.FinishReasonPath = cfg.paths[len(cfg.paths)-1].Path
	}
	return cfg
}

func resolveFinishReasonModeRegistry(pathByMode map[string]string, raw finishReasonModeRegistry) (finishReasonModeRegistry, error) {
	merged := finishReasonModeRegistry{}
	paths := map[string]string{}
	for name, cfg := range raw {
		merged[name] = cfg
	}
	for name, path := range pathByMode {
		paths[name] = path
	}
	for name, cfg := range merged {
		if strings.TrimSpace(cfg.Mode) == "" && cfg.hasFinishReasonPath() {
			cfg.Mode = usageModeCustom
			merged[name] = cfg
		}
	}
	resolved := finishReasonModeRegistry{}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg, err := resolveFinishReasonExtractConfig(paths[name], "", fmt.Sprintf("finish_reason_mode %q", name), merged[name], merged, nil)
		if err != nil {
			return nil, err
		}
		if err := validateResolvedFinishReasonExtractConfig(paths[name], "", fmt.Sprintf("finish_reason_mode %q", name), cfg); err != nil {
			return nil, err
		}
		resolved[name] = normalizeFinishReasonExtractConfig(cfg)
	}
	return resolved, nil
}

func resolveFinishReasonExtractConfig(path, providerName, scope string, cfg FinishReasonExtractConfig, registry finishReasonModeRegistry, stack []string) (FinishReasonExtractConfig, error) {
	mode := normalizeFinishReasonMode(cfg.Mode)
	switch mode {
	case "":
		if cfg.hasFinishReasonPath() {
			cfg.Mode = usageModeCustom
		}
		return normalizeFinishReasonExtractConfig(cfg), nil
	case usageModeCustom:
		return normalizeFinishReasonExtractConfig(cfg), nil
	}
	if baseCfg, ok := registry[mode]; ok {
		for _, item := range stack {
			if item == mode {
				return FinishReasonExtractConfig{}, fmt.Errorf("%s in %q: recursive finish_reason_mode reference %q", scope, path, mode)
			}
		}
		base, err := resolveFinishReasonExtractConfig(path, providerName, fmt.Sprintf("finish_reason_mode %q", mode), baseCfg, registry, append(stack, mode))
		if err != nil {
			return FinishReasonExtractConfig{}, err
		}
		override := cfg
		override.Mode = ""
		return normalizeFinishReasonExtractConfig(mergeFinishReasonConfig(base, override)), nil
	}
	return FinishReasonExtractConfig{}, validationIssue(
		fmt.Errorf("provider %q in %q: %s unsupported finish_reason_extract mode %q", providerName, path, scope, cfg.Mode),
		scope,
		"finish_reason_extract",
	)
}
