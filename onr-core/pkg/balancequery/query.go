package balancequery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

type Params struct {
	Provider string
	File     dslconfig.ProviderFile
	Meta     dslmeta.Meta
	BaseURL  string
	APIKey   string

	HTTPClient *http.Client
	DebugOut   io.Writer
}

type Result struct {
	Provider string
	Mode     string
	Unit     string
	Balance  float64
	Used     *float64
}

type openAISubscriptionResponse struct {
	HasPaymentMethod bool    `json:"has_payment_method"`
	HardLimitUSD     float64 `json:"hard_limit_usd"`
}

type openAIUsageResponse struct {
	TotalUsage float64 `json:"total_usage"`
}

func Query(ctx context.Context, p Params) (Result, error) {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider == "" {
		return Result{}, errors.New("provider is empty")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return Result{}, errors.New("api key is empty")
	}

	meta := p.Meta
	meta.API = strings.TrimSpace(meta.API)
	if meta.API == "" {
		return Result{}, errors.New("meta.api is empty")
	}
	meta.APIKey = strings.TrimSpace(p.APIKey)

	cfgBalance, ok := p.File.Balance.Select(&meta)
	if !ok {
		return Result{}, fmt.Errorf("provider %q has no balance config for api=%q stream=%v", provider, meta.API, meta.IsStream)
	}

	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = resolveBaseURLFromExpr(p.File.Routing.BaseURLExpr)
	}
	if baseURL == "" {
		return Result{}, errors.New("base url is empty")
	}
	meta.BaseURL = baseURL

	headers := make(http.Header)
	p.File.Headers.Apply(&meta, headers)
	applyHeaderOps(headers, cfgBalance.Headers, &meta)

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	if ctx == nil {
		ctx = context.Background()
	}

	mode := strings.ToLower(strings.TrimSpace(cfgBalance.Mode))
	var (
		balance float64
		used    *float64
		err     error
	)
	switch mode {
	case "openai":
		balance, used, err = queryOpenAIBalanceWithPaths(ctx, client, baseURL, p.APIKey, cfgBalance.SubscriptionPath, cfgBalance.UsagePath, headers, p.DebugOut)
	case "custom":
		balance, used, err = queryCustomBalance(ctx, client, baseURL, cfgBalance, headers, p.DebugOut)
	default:
		return Result{}, fmt.Errorf("unsupported balance mode %q", cfgBalance.Mode)
	}
	if err != nil {
		return Result{}, err
	}

	unit := strings.TrimSpace(cfgBalance.Unit)
	if unit == "" {
		unit = "USD"
	}
	return Result{
		Provider: provider,
		Mode:     mode,
		Unit:     unit,
		Balance:  balance,
		Used:     used,
	}, nil
}

func queryCustomBalance(ctx context.Context, client *http.Client, baseURL string, cfg dslconfig.BalanceQueryConfig, headers http.Header, debugOut io.Writer) (float64, *float64, error) {
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	reqURL, err := buildBalanceRequestURL(baseURL, "", cfg.Path)
	if err != nil {
		return 0, nil, err
	}
	body, err := getResponseBody(ctx, client, method, reqURL, headers, debugOut)
	if err != nil {
		return 0, nil, err
	}
	return dslconfig.ExtractBalance(cfg, body)
}

func queryOpenAIBalanceWithPaths(ctx context.Context, client *http.Client, baseURL, apiKey, subscriptionPath, usagePath string, baseHeaders http.Header, debugOut io.Writer) (float64, *float64, error) {
	subURL, err := buildBalanceRequestURL(baseURL, "/v1/dashboard/billing/subscription", subscriptionPath)
	if err != nil {
		return 0, nil, err
	}
	subHeaders := cloneHeaders(baseHeaders)
	if strings.TrimSpace(subHeaders.Get("Authorization")) == "" {
		subHeaders.Set("Authorization", "Bearer "+apiKey)
	}
	body, err := getResponseBody(ctx, client, http.MethodGet, subURL, subHeaders, debugOut)
	if err != nil {
		return 0, nil, err
	}
	subscription := openAISubscriptionResponse{}
	if err := json.Unmarshal(body, &subscription); err != nil {
		return 0, nil, err
	}

	now := time.Now()
	startDate := fmt.Sprintf("%s-01", now.Format("2006-01"))
	endDate := now.Format("2006-01-02")
	if !subscription.HasPaymentMethod {
		startDate = now.AddDate(0, 0, -100).Format("2006-01-02")
	}

	uPath := strings.TrimSpace(usagePath)
	if uPath == "" {
		uPath = "/v1/dashboard/billing/usage"
	}
	sep := "?"
	if strings.Contains(uPath, "?") {
		sep = "&"
	}
	uPath = fmt.Sprintf("%s%vstart_date=%s&end_date=%s", uPath, sep, startDate, endDate)

	usageURL, err := buildBalanceRequestURL(baseURL, "/v1/dashboard/billing/usage", uPath)
	if err != nil {
		return 0, nil, err
	}
	usageHeaders := cloneHeaders(baseHeaders)
	if strings.TrimSpace(usageHeaders.Get("Authorization")) == "" {
		usageHeaders.Set("Authorization", "Bearer "+apiKey)
	}
	body, err = getResponseBody(ctx, client, http.MethodGet, usageURL, usageHeaders, debugOut)
	if err != nil {
		return 0, nil, err
	}
	usage := openAIUsageResponse{}
	if err := json.Unmarshal(body, &usage); err != nil {
		return 0, nil, err
	}

	balance := subscription.HardLimitUSD - usage.TotalUsage/100
	used := usage.TotalUsage / 100
	return balance, &used, nil
}

func getResponseBody(ctx context.Context, client *http.Client, method, reqURL string, headers http.Header, debugOut io.Writer) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	mergeHeaders(req.Header, headers)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if debugOut != nil {
		_, _ = fmt.Fprintf(debugOut, "debug upstream_response method=%s url=%s status=%d body=%s\n", method, reqURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request %s %s failed: status=%d body=%s", method, reqURL, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func resolveBaseURLFromExpr(expr string) string {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err == nil {
			return strings.TrimSpace(v)
		}
	}
	return raw
}

func mergeHeaders(dst, src http.Header) {
	for k, vals := range src {
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

func cloneHeaders(h http.Header) http.Header {
	out := make(http.Header, len(h))
	for k, vals := range h {
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[k] = cp
	}
	return out
}

func buildBalanceRequestURL(baseURL, defaultPath, configuredPath string) (string, error) {
	path := strings.TrimSpace(configuredPath)
	if path == "" {
		path = strings.TrimSpace(defaultPath)
	}
	if path == "" {
		return "", errors.New("balance path is empty")
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	b := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if b == "" {
		return "", errors.New("balance baseURL is empty")
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u, err := url.Parse(b + path)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func applyHeaderOps(h http.Header, ops []dslconfig.HeaderOp, meta *dslmeta.Meta) {
	for _, op := range ops {
		name := strings.TrimSpace(evalHeaderExpr(op.NameExpr, meta))
		if name == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(op.Op)) {
		case "header_set":
			h.Set(name, evalHeaderExpr(op.ValueExpr, meta))
		case "header_del":
			h.Del(name)
		}
	}
}

func evalHeaderExpr(expr string, meta *dslmeta.Meta) string {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")")
		parts := splitTopLevelArgs(inner)
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(evalHeaderExpr(p, meta))
		}
		return b.String()
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		v, err := strconv.Unquote(raw)
		if err == nil {
			return v
		}
		return raw
	}
	switch raw {
	case "$channel.base_url":
		if meta != nil {
			return meta.BaseURL
		}
	case "$channel.key":
		if meta != nil {
			return meta.APIKey
		}
	case "$request.model":
		if meta != nil {
			return meta.ActualModelName
		}
	case "$request.model_mapped":
		if meta != nil {
			if meta.DSLModelMapped != "" {
				return meta.DSLModelMapped
			}
			return meta.ActualModelName
		}
	}
	return raw
}

func splitTopLevelArgs(s string) []string {
	var parts []string
	var b strings.Builder
	depth := 0
	inString := false
	escaped := false
	flush := func() {
		p := strings.TrimSpace(b.String())
		b.Reset()
		if p != "" {
			parts = append(parts, p)
		}
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inString {
			b.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
			b.WriteByte(ch)
		case '(':
			depth++
			b.WriteByte(ch)
		case ')':
			if depth > 0 {
				depth--
			}
			b.WriteByte(ch)
		case ',':
			if depth == 0 {
				flush()
				continue
			}
			b.WriteByte(ch)
		default:
			b.WriteByte(ch)
		}
	}
	flush()
	return parts
}
