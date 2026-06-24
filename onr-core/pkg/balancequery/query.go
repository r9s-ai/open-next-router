package balancequery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslquery"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient"
)

type Params struct {
	Provider string
	File     dslconfig.ProviderFile
	Meta     *dslmeta.Meta
	BaseURL  string
	APIKey   string

	// HTTPClient lets next-router supply a fake client, ensuring tests can run
	// without hitting upstream HTTP services.
	HTTPClient httpclient.HTTPDoer
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

// Query requires a non-nil ctx and a non-empty p.Meta.API.
// It returns a populated Result on success.
func Query(ctx context.Context, p Params) (Result, error) {
	provider := strings.ToLower(strings.TrimSpace(p.Provider))
	if provider == "" {
		return Result{}, errors.New("provider is empty")
	}
	if strings.TrimSpace(p.APIKey) == "" {
		return Result{}, errors.New("api key is empty")
	}

	meta := dslmeta.Clone(p.Meta)
	meta.API = strings.TrimSpace(meta.API)
	if meta.API == "" {
		return Result{}, errors.New("meta.api is empty")
	}
	meta.APIKey = strings.TrimSpace(p.APIKey)

	cfgBalance, ok := p.File.Balance.Select(meta)
	if !ok {
		return Result{}, fmt.Errorf("provider %q has no balance config for api=%q stream=%v", provider, meta.API, meta.IsStream)
	}

	baseURL := strings.TrimSpace(p.BaseURL)
	if baseURL == "" {
		baseURL = dslquery.ResolveBaseURLFromExpr(p.File.Routing.BaseURLExpr)
	}
	if baseURL == "" {
		return Result{}, errors.New("base url is empty")
	}
	meta.BaseURL = baseURL

	headers := make(http.Header)
	p.File.Headers.Apply(meta, nil, headers)
	dslquery.ApplyHeaderOps(headers, cfgBalance.Headers, meta)

	client := p.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	mode := strings.ToLower(strings.TrimSpace(cfgBalance.Mode))
	var (
		balance float64
		used    *float64
		err     error
	)
	switch mode {
	case "openai":
		balance, used, err = queryOpenAIBalanceWithPaths(ctx, client, baseURL, p.APIKey, cfgBalance.SubscriptionPath, cfgBalance.UsagePath, headers, meta, p.DebugOut)
	case "custom":
		balance, used, err = queryCustomBalance(ctx, client, baseURL, cfgBalance, headers, meta, p.DebugOut)
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

func queryCustomBalance(ctx context.Context, client httpclient.HTTPDoer, baseURL string, cfg *dslconfig.BalanceQueryConfig, headers http.Header, meta *dslmeta.Meta, debugOut io.Writer) (float64, *float64, error) {
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = http.MethodGet
	}
	reqURL, err := dslquery.BuildRequestURL(baseURL, "", dslconfig.EvalStringExpr(cfg.Path, meta), "balance")
	if err != nil {
		return 0, nil, err
	}
	body, err := dslquery.GetResponseBody(ctx, client, method, reqURL, headers, debugOut)
	if err != nil {
		return 0, nil, err
	}
	return dslconfig.ExtractBalance(cfg, body)
}

func queryOpenAIBalanceWithPaths(ctx context.Context, client httpclient.HTTPDoer, baseURL, apiKey, subscriptionPath, usagePath string, baseHeaders http.Header, meta *dslmeta.Meta, debugOut io.Writer) (float64, *float64, error) {
	subURL, err := dslquery.BuildRequestURL(baseURL, "/v1/dashboard/billing/subscription", dslconfig.EvalStringExpr(subscriptionPath, meta), "balance")
	if err != nil {
		return 0, nil, err
	}
	subHeaders := dslquery.CloneHeaders(baseHeaders)
	if strings.TrimSpace(subHeaders.Get("Authorization")) == "" {
		subHeaders.Set("Authorization", "Bearer "+apiKey)
	}
	body, err := dslquery.GetResponseBody(ctx, client, http.MethodGet, subURL, subHeaders, debugOut)
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

	uPath := strings.TrimSpace(dslconfig.EvalStringExpr(usagePath, meta))
	if uPath == "" {
		uPath = "/v1/dashboard/billing/usage"
	}
	sep := "?"
	if strings.Contains(uPath, "?") {
		sep = "&"
	}
	uPath = fmt.Sprintf("%s%vstart_date=%s&end_date=%s", uPath, sep, startDate, endDate)

	usageURL, err := dslquery.BuildRequestURL(baseURL, "/v1/dashboard/billing/usage", uPath, "balance")
	if err != nil {
		return 0, nil, err
	}
	usageHeaders := dslquery.CloneHeaders(baseHeaders)
	if strings.TrimSpace(usageHeaders.Get("Authorization")) == "" {
		usageHeaders.Set("Authorization", "Bearer "+apiKey)
	}
	body, err = dslquery.GetResponseBody(ctx, client, http.MethodGet, usageURL, usageHeaders, debugOut)
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
