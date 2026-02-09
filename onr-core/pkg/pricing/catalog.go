package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	DefaultCatalogURL = "https://models.dev/api.json"
	defaultUserAgent  = "open-next-router-onr-admin/pricing-sync"
)

type Catalog struct {
	Providers map[string]Provider
}

type Provider struct {
	ID     string
	Name   string
	Models map[string]Model
}

type Model struct {
	ID   string
	Name string
	Cost map[string]float64
}

type FetchResult struct {
	Catalog   Catalog
	URL       string
	FetchedAt time.Time
}

type ModelPrice struct {
	Provider string
	Model    string
	Cost     map[string]float64
}

type apiProvider struct {
	ID     string              `json:"id"`
	Name   string              `json:"name"`
	Models map[string]apiModel `json:"models"`
}

type apiModel struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Cost map[string]any `json:"cost"`
}

var providerAliases = map[string]string{
	"gemini":         "google",
	"moonshot":       "moonshotai-cn",
	"moonshotai":     "moonshotai-cn",
	"azure-response": "azure",
}

func FetchCatalog(ctx context.Context, client *http.Client, url string) (*FetchResult, error) {
	rawURL := strings.TrimSpace(url)
	if rawURL == "" {
		rawURL = DefaultCatalogURL
	}
	hc := client
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("fetch catalog failed: status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var raw map[string]apiProvider
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode catalog failed: %w", err)
	}

	out := Catalog{
		Providers: map[string]Provider{},
	}
	for pid, p := range raw {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			id = strings.TrimSpace(pid)
		}
		if id == "" {
			continue
		}
		models := map[string]Model{}
		for mid, m := range p.Models {
			modelID := strings.TrimSpace(m.ID)
			if modelID == "" {
				modelID = strings.TrimSpace(mid)
			}
			if modelID == "" {
				continue
			}
			models[modelID] = Model{
				ID:   modelID,
				Name: strings.TrimSpace(m.Name),
				Cost: normalizeCostMap(m.Cost),
			}
		}
		out.Providers[id] = Provider{
			ID:     id,
			Name:   strings.TrimSpace(p.Name),
			Models: models,
		}
	}

	return &FetchResult{
		Catalog:   out,
		URL:       rawURL,
		FetchedAt: time.Now().UTC(),
	}, nil
}

func (c Catalog) ResolveProviderID(input string) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(input))
	if name == "" {
		return "", false
	}
	if _, ok := c.Providers[name]; ok {
		return name, true
	}
	if alias, ok := providerAliases[name]; ok {
		if _, found := c.Providers[alias]; found {
			return alias, true
		}
	}
	return "", false
}

func (c Catalog) ExtractPrices(providerInput string, modelInputs []string) ([]ModelPrice, string, error) {
	providerID, ok := c.ResolveProviderID(providerInput)
	if !ok {
		return nil, "", fmt.Errorf("provider not found in models.dev: %s", strings.TrimSpace(providerInput))
	}
	p := c.Providers[providerID]
	if len(p.Models) == 0 {
		return nil, providerID, fmt.Errorf("provider has no models: %s", providerID)
	}

	wantModels := make([]string, 0, len(modelInputs))
	seen := map[string]struct{}{}
	for _, model := range modelInputs {
		m := strings.TrimSpace(model)
		if m == "" {
			continue
		}
		if _, exists := seen[m]; exists {
			continue
		}
		seen[m] = struct{}{}
		wantModels = append(wantModels, m)
	}

	var selected []string
	if len(wantModels) == 0 {
		selected = make([]string, 0, len(p.Models))
		for modelID := range p.Models {
			selected = append(selected, modelID)
		}
	} else {
		selected = wantModels
	}
	sort.Strings(selected)

	out := make([]ModelPrice, 0, len(selected))
	for _, modelID := range selected {
		m, exists := p.Models[modelID]
		if !exists {
			return nil, providerID, fmt.Errorf("model not found in provider %s: %s", providerID, modelID)
		}
		out = append(out, ModelPrice{
			Provider: providerID,
			Model:    modelID,
			Cost:     cloneFloatMap(m.Cost),
		})
	}
	return out, providerID, nil
}

func normalizeCostMap(in map[string]any) map[string]float64 {
	if len(in) == 0 {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(in))
	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		n, ok := coerceNumber(v)
		if !ok {
			continue
		}
		out[key] = n
	}
	return out
}

func coerceNumber(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case int32:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func cloneFloatMap(in map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
