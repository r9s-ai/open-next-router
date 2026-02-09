package dslconfig

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
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

func (p ProviderBalance) Select(meta *dslmeta.Meta) (BalanceQueryConfig, bool) {
	if meta == nil {
		return BalanceQueryConfig{}, false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return BalanceQueryConfig{}, false
	}
	cfg := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		cfg = mergeBalanceConfig(cfg, m.Query)
	}
	if strings.TrimSpace(cfg.Mode) == "" {
		return BalanceQueryConfig{}, false
	}
	return cfg, true
}

func (p ProviderBalance) selectMatch(api string, stream bool) (MatchBalance, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return MatchBalance{}, false
}

func mergeBalanceConfig(base BalanceQueryConfig, override BalanceQueryConfig) BalanceQueryConfig {
	out := base
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
	return out
}

// ExtractBalance parses custom balance response and extracts balance/used values.
func ExtractBalance(cfg BalanceQueryConfig, respBody []byte) (float64, *float64, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
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
	if strings.TrimSpace(cfg.BalanceExpr) == "" && strings.TrimSpace(cfg.BalancePath) == "" {
		return 0, nil, fmt.Errorf("balance field is required")
	}

	var used *float64
	if strings.TrimSpace(cfg.UsedExpr) != "" || strings.TrimSpace(cfg.UsedPath) != "" {
		v, err := evalBalanceField(root, cfg.UsedExpr, cfg.UsedPath)
		if err != nil {
			return 0, nil, err
		}
		used = &v
	}
	return balance, used, nil
}

func evalBalanceField(root map[string]any, expr, path string) (float64, error) {
	if strings.TrimSpace(expr) != "" {
		parsed, err := ParseBalanceExpr(expr)
		if err != nil {
			return 0, fmt.Errorf("invalid balance expr %q: %w", expr, err)
		}
		return parsed.Eval(root), nil
	}
	p := strings.TrimSpace(path)
	if p == "" {
		return 0, nil
	}
	return getFloatByPath(root, p), nil
}

func getFloatByPath(root map[string]any, path string) float64 {
	p := strings.TrimSpace(path)
	if p == "" || !strings.HasPrefix(p, "$.") {
		return 0
	}
	parts := strings.Split(strings.TrimPrefix(p, "$."), ".")
	vals, ok := collectPathValues(root, parts)
	if !ok {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += coerceFloat(v)
	}
	return sum
}

func coerceFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case int32:
		return float64(t)
	case json.Number:
		f, err := t.Float64()
		if err == nil {
			return f
		}
		i, err := t.Int64()
		if err == nil {
			return float64(i)
		}
		return 0
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
