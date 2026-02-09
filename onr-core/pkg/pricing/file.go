package pricing

import (
	"sort"
	"strings"
	"time"
)

const defaultPriceUnit = "usd_per_1m_tokens"

type PriceFile struct {
	Version string      `yaml:"version"`
	Source  SourceInfo  `yaml:"source"`
	Unit    string      `yaml:"unit"`
	Entries []PriceItem `yaml:"entries"`
}

type SourceInfo struct {
	URL       string   `yaml:"url"`
	FetchedAt string   `yaml:"fetched_at"`
	Providers []string `yaml:"providers"`
}

type PriceItem struct {
	Model    string             `yaml:"model"`
	Provider string             `yaml:"provider"`
	Cost     map[string]float64 `yaml:"cost"`
}

func BuildPriceFile(fetch *FetchResult, providerIDs []string, prices []ModelPrice) PriceFile {
	normalizedProviders := make([]string, 0, len(providerIDs))
	seen := map[string]struct{}{}
	for _, provider := range providerIDs {
		p := strings.TrimSpace(provider)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		normalizedProviders = append(normalizedProviders, p)
	}
	sort.Strings(normalizedProviders)

	entries := make([]PriceItem, 0, len(prices))
	for _, p := range prices {
		entries = append(entries, PriceItem{
			Model:    strings.TrimSpace(p.Model),
			Provider: strings.TrimSpace(p.Provider),
			Cost:     cloneFloatMap(p.Cost),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Provider != entries[j].Provider {
			return entries[i].Provider < entries[j].Provider
		}
		return entries[i].Model < entries[j].Model
	})

	url := ""
	fetchedAt := time.Now().UTC().Format(time.RFC3339)
	if fetch != nil {
		url = strings.TrimSpace(fetch.URL)
		if !fetch.FetchedAt.IsZero() {
			fetchedAt = fetch.FetchedAt.UTC().Format(time.RFC3339)
		}
	}
	return PriceFile{
		Version: "v1",
		Source: SourceInfo{
			URL:       url,
			FetchedAt: fetchedAt,
			Providers: normalizedProviders,
		},
		Unit:    defaultPriceUnit,
		Entries: entries,
	}
}
