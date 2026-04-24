package dslconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const (
	balanceModeOpenAI = "openai"
	balanceModeCustom = "custom"
)

type BalanceQueryConfig struct {
	Mode string `json:"mode,omitempty"`

	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`

	BalancePath string `json:"balance_path,omitempty"`
	BalanceExpr string `json:"balance_expr,omitempty"`
	UsedPath    string `json:"used_path,omitempty"`
	UsedExpr    string `json:"used_expr,omitempty"`

	Unit string `json:"unit,omitempty"`

	SubscriptionPath string `json:"subscription_path,omitempty"`
	UsagePath        string `json:"usage_path,omitempty"`

	Headers []HeaderOp `json:"headers,omitempty"`
}

type ProviderBalance struct {
	Defaults BalanceQueryConfig
	Matches  []MatchBalance
}

type MatchBalance struct {
	API    string
	Stream *bool

	Query BalanceQueryConfig
}

// Select requires a non-nil meta and a valid ProviderBalance receiver.
func (p *ProviderBalance) Select(meta *dslmeta.Meta) (*BalanceQueryConfig, bool) {
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return nil, false
	}
	cfg := inferImplicitCustomBalanceQueryConfig(&p.Defaults)
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		cfg = inferImplicitCustomBalanceQueryConfig(mergeBalanceConfig(cfg, &m.Query))
	}
	if cfg.Mode == "" {
		return nil, false
	}
	return cfg, true
}

func (p *ProviderBalance) selectMatch(api string, stream bool) (*MatchBalance, bool) {
	for i := range p.Matches {
		m := &p.Matches[i]
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return nil, false
}

func mergeBalanceConfig(base *BalanceQueryConfig, override *BalanceQueryConfig) *BalanceQueryConfig {
	out := *base
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.Method) != "" {
		out.Method = override.Method
	}
	if strings.TrimSpace(override.Path) != "" {
		out.Path = override.Path
	}
	if strings.TrimSpace(override.BalancePath) != "" {
		out.BalancePath = override.BalancePath
	}
	if strings.TrimSpace(override.BalanceExpr) != "" {
		out.BalanceExpr = override.BalanceExpr
	}
	if strings.TrimSpace(override.UsedPath) != "" {
		out.UsedPath = override.UsedPath
	}
	if strings.TrimSpace(override.UsedExpr) != "" {
		out.UsedExpr = override.UsedExpr
	}
	if strings.TrimSpace(override.Unit) != "" {
		out.Unit = override.Unit
	}
	if strings.TrimSpace(override.SubscriptionPath) != "" {
		out.SubscriptionPath = override.SubscriptionPath
	}
	if strings.TrimSpace(override.UsagePath) != "" {
		out.UsagePath = override.UsagePath
	}
	if len(override.Headers) > 0 {
		out.Headers = append([]HeaderOp(nil), override.Headers...)
	}
	return normalizeBalanceQueryConfig(&out)
}

func normalizeBalanceQueryConfig(in *BalanceQueryConfig) *BalanceQueryConfig {
	out := *in
	out.Mode = strings.ToLower(strings.TrimSpace(out.Mode))
	if out.Method == "" {
		out.Method = "GET"
	}
	return &out
}

func inferImplicitCustomBalanceQueryConfig(in *BalanceQueryConfig) *BalanceQueryConfig {
	out := normalizeBalanceQueryConfig(in)
	if out.Mode == "" && hasAnyBalanceQueryRule(in) {
		out.Mode = balanceModeCustom
	}
	return out
}

func hasAnyBalanceQueryRule(cfg *BalanceQueryConfig) bool {
	return strings.TrimSpace(cfg.Path) != "" ||
		strings.TrimSpace(cfg.BalancePath) != "" ||
		strings.TrimSpace(cfg.BalanceExpr) != "" ||
		strings.TrimSpace(cfg.UsedPath) != "" ||
		strings.TrimSpace(cfg.UsedExpr) != "" ||
		strings.TrimSpace(cfg.Unit) != "" ||
		strings.TrimSpace(cfg.SubscriptionPath) != "" ||
		strings.TrimSpace(cfg.UsagePath) != "" ||
		len(cfg.Headers) > 0
}

// ExtractBalance parses custom balance response and extracts balance/used values.
func ExtractBalance(cfg *BalanceQueryConfig, respBody []byte) (float64, *float64, error) {
	cfg = inferImplicitCustomBalanceQueryConfig(cfg)
	mode := cfg.Mode
	if mode != balanceModeCustom {
		return 0, nil, fmt.Errorf("unsupported balance mode %q", cfg.Mode)
	}

	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return 0, nil, fmt.Errorf("parse response json: %w", err)
	}
	root, ok := data.(map[string]any)
	if !ok {
		return 0, nil, fmt.Errorf("response is not json object")
	}

	balance, err := evalBalanceField(root, cfg.BalanceExpr, cfg.BalancePath)
	if err != nil {
		return 0, nil, err
	}
	if cfg.BalanceExpr == "" && cfg.BalancePath == "" {
		return 0, nil, fmt.Errorf("balance field is required")
	}

	var used *float64
	if cfg.UsedExpr != "" || cfg.UsedPath != "" {
		v, err := evalBalanceField(root, cfg.UsedExpr, cfg.UsedPath)
		if err != nil {
			return 0, nil, err
		}
		used = &v
	}
	return balance, used, nil
}

func evalBalanceField(root map[string]any, expr, path string) (float64, error) {
	if expr != "" {
		parsed, err := ParseBalanceExpr(expr)
		if err != nil {
			return 0, fmt.Errorf("invalid balance expr %q: %w", expr, err)
		}
		return parsed.Eval(root), nil
	}
	if path == "" {
		return 0, nil
	}
	return jsonutil.GetFloatByPath(root, path), nil
}
