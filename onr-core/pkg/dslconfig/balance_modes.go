package dslconfig

import (
	"fmt"
	"sort"
	"strings"
)

type balanceModeRegistry map[string]BalanceQueryConfig

func parseGlobalBalanceModes(path string, content string) (balanceModeRegistry, error) {
	s := newScanner(path, content)
	modes := balanceModeRegistry{}
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return modes, nil
		case tokIdent:
			switch tok.text {
			case "balance_mode":
				name, cfg, err := parseBalanceModeBlock(s)
				if err != nil {
					return nil, err
				}
				if _, exists := modes[name]; exists {
					return nil, fmt.Errorf("duplicate balance_mode %q in %q", name, path)
				}
				modes[name] = cfg
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return nil, err
				}
			}
		}
	}
}

func parseBalanceModeBlock(s *scanner) (string, BalanceQueryConfig, error) {
	nameTok := s.nextNonTrivia()
	switch nameTok.kind {
	case tokIdent, tokString:
	default:
		return "", BalanceQueryConfig{}, s.errAt(nameTok, "balance_mode expects mode name")
	}
	name := nameTok.text
	if nameTok.kind == tokString {
		name = unquoteString(nameTok.text)
	}
	name = normalizeUsageModeName(name)
	if name == "" {
		return "", BalanceQueryConfig{}, s.errAt(nameTok, "balance_mode name is empty")
	}
	lb := s.nextNonTrivia()
	if lb.kind != tokLBrace {
		return "", BalanceQueryConfig{}, s.errAt(lb, "expected '{' after balance_mode name")
	}

	var cfg BalanceQueryConfig
	var hdr PhaseHeaders
	for {
		tok := s.nextNonTrivia()
		switch tok.kind {
		case tokEOF:
			return "", BalanceQueryConfig{}, s.errAt(tok, "unexpected EOF in balance_mode block")
		case tokRBrace:
			if len(hdr.Request) > 0 {
				cfg.Headers = append(cfg.Headers, hdr.Request...)
			}
			return name, cfg, nil
		case tokIdent:
			switch tok.text {
			case "balance_mode":
				mode, err := parseModeArgStmt(s, "balance_mode")
				if err != nil {
					return "", BalanceQueryConfig{}, err
				}
				cfg.Mode = strings.TrimSpace(mode)
			case "method", "path", "balance_path", "used_path", "balance_unit", "subscription_path", "usage_path":
				v, err := parseBalanceFieldStmt(s, tok.text)
				if err != nil {
					return "", BalanceQueryConfig{}, err
				}
				switch tok.text {
				case "method":
					cfg.Method = v
				case "path":
					cfg.Path = v
				case "balance_path":
					cfg.BalancePath = v
				case "used_path":
					cfg.UsedPath = v
				case "balance_unit":
					cfg.Unit = v
				case "subscription_path":
					cfg.SubscriptionPath = v
				case "usage_path":
					cfg.UsagePath = v
				}
			case balanceExprKey, usedExprKey:
				if err := consumeEquals(s); err != nil {
					return "", BalanceQueryConfig{}, err
				}
				expr, err := consumeExprUntilSemicolon(s)
				if err != nil {
					return "", BalanceQueryConfig{}, err
				}
				expr = strings.TrimSpace(expr)
				if tok.text == balanceExprKey {
					cfg.BalanceExpr = expr
				} else {
					cfg.UsedExpr = expr
				}
			case "set_header":
				if err := parseSetHeaderStmt(s, &hdr); err != nil {
					return "", BalanceQueryConfig{}, err
				}
			case "del_header":
				if err := parseDelHeaderStmt(s, &hdr); err != nil {
					return "", BalanceQueryConfig{}, err
				}
			default:
				if err := skipStmtOrBlock(s); err != nil {
					return "", BalanceQueryConfig{}, err
				}
			}
		}
	}
}

func resolveBalanceModeRegistry(pathByMode map[string]string, raw balanceModeRegistry) (balanceModeRegistry, error) {
	merged := balanceModeRegistry{}
	paths := map[string]string{}
	for name, cfg := range raw {
		merged[name] = cfg
	}
	for name, path := range pathByMode {
		paths[name] = path
	}
	for name, cfg := range merged {
		if strings.TrimSpace(cfg.Mode) == "" {
			if builtin := builtinBalancePresetName(name); builtin != "" {
				cfg.Mode = builtin
			} else if hasAnyBalanceQueryRule(&cfg) {
				cfg.Mode = balanceModeCustom
			}
			merged[name] = *normalizeBalanceQueryConfig(&cfg)
		}
	}
	resolved := balanceModeRegistry{}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg, err := resolveBalanceQueryConfig(paths[name], "", fmt.Sprintf("balance_mode %q", name), merged[name], merged, nil)
		if err != nil {
			return nil, err
		}
		if err := validateResolvedBalanceQueryConfig(paths[name], "", fmt.Sprintf("balance_mode %q", name), cfg); err != nil {
			return nil, err
		}
		resolved[name] = cfg
	}
	return resolved, nil
}

func resolveBalanceQueryConfig(path, providerName, scope string, cfg BalanceQueryConfig, registry balanceModeRegistry, stack []string) (BalanceQueryConfig, error) {
	mode := normalizeUsageMode(cfg.Mode)
	switch mode {
	case "":
		return *inferImplicitCustomBalanceQueryConfig(&cfg), nil
	case balanceModeCustom:
		return *normalizeBalanceQueryConfig(&cfg), nil
	}
	if builtinBalancePresetName(mode) != "" && len(stack) > 0 && stack[len(stack)-1] == mode {
		return *normalizeBalanceQueryConfig(&cfg), nil
	}
	if baseCfg, ok := registry[mode]; ok {
		for _, item := range stack {
			if item == mode {
				return BalanceQueryConfig{}, fmt.Errorf("%s in %q: recursive balance_mode reference %q", scope, path, mode)
			}
		}
		base, err := resolveBalanceQueryConfig(path, providerName, fmt.Sprintf("balance_mode %q", mode), baseCfg, registry, append(stack, mode))
		if err != nil {
			return BalanceQueryConfig{}, err
		}
		override := cfg
		override.Mode = ""
		return *mergeBalanceConfig(&base, &override), nil
	}
	if (providerName == "" || len(stack) > 0) && builtinBalancePresetName(mode) != "" {
		return *normalizeBalanceQueryConfig(&cfg), nil
	}
	return BalanceQueryConfig{}, validationIssue(
		fmt.Errorf("provider %q in %q: %s unsupported balance_mode %q", providerName, path, scope, cfg.Mode),
		scope,
		"balance_mode",
	)
}

func builtinBalancePresetName(mode string) string {
	switch normalizeUsageMode(mode) {
	case balanceModeOpenAI:
		return balanceModeOpenAI
	default:
		return ""
	}
}
