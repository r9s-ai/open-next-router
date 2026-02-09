package pricing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testProviderOpenAI = "openai"
	testProviderGoogle = "google"
)

func TestFetchCatalogAndExtractPrices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatalf("missing user-agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "openai": {
    "id": "openai",
    "name": "OpenAI",
    "models": {
      "gpt-4o-mini": {
        "id": "gpt-4o-mini",
        "name": "GPT-4o mini",
        "cost": {"input": 0.15, "output": 0.6, "cache_read": 0.08}
      }
    }
  },
  "google": {
    "id": "google",
    "name": "Google",
    "models": {
      "gemini-2.5-flash": {
        "id": "gemini-2.5-flash",
        "cost": {"input": 0.3, "output": 2.5}
      }
    }
  }
}`))
	}))
	defer srv.Close()

	out, err := FetchCatalog(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatalf("FetchCatalog error: %v", err)
	}
	if len(out.Catalog.Providers) != 2 {
		t.Fatalf("providers len=%d want=2", len(out.Catalog.Providers))
	}

	prices, providerID, err := out.Catalog.ExtractPrices(testProviderOpenAI, []string{"gpt-4o-mini"})
	if err != nil {
		t.Fatalf("ExtractPrices openai error: %v", err)
	}
	if providerID != testProviderOpenAI {
		t.Fatalf("providerID=%q want=%s", providerID, testProviderOpenAI)
	}
	if len(prices) != 1 {
		t.Fatalf("prices len=%d want=1", len(prices))
	}
	if prices[0].Cost["input"] != 0.15 {
		t.Fatalf("input cost=%v want=0.15", prices[0].Cost["input"])
	}

	prices, providerID, err = out.Catalog.ExtractPrices("gemini", []string{"gemini-2.5-flash"})
	if err != nil {
		t.Fatalf("ExtractPrices gemini alias error: %v", err)
	}
	if providerID != testProviderGoogle {
		t.Fatalf("providerID=%q want=%s", providerID, testProviderGoogle)
	}
	if len(prices) != 1 {
		t.Fatalf("prices len=%d want=1", len(prices))
	}
	if prices[0].Cost["output"] != 2.5 {
		t.Fatalf("output cost=%v want=2.5", prices[0].Cost["output"])
	}
}

func TestExtractPricesModelNotFound(t *testing.T) {
	c := Catalog{
		Providers: map[string]Provider{
			testProviderOpenAI: {
				ID: testProviderOpenAI,
				Models: map[string]Model{
					"gpt-4o-mini": {ID: "gpt-4o-mini", Cost: map[string]float64{"input": 0.15}},
				},
			},
		},
	}
	_, _, err := c.ExtractPrices(testProviderOpenAI, []string{"missing-model"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
