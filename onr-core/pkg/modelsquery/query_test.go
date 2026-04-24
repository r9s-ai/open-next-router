package modelsquery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient/httpclienttest"
)

func TestQuery_CustomModelsConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("auth header=%q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
	}))
	defer srv.Close()

	pf := dslconfig.ProviderFile{
		Routing: dslconfig.ProviderRouting{
			BaseURLExpr: `"` + srv.URL + `"`,
		},
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Auth: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"Authorization"`,
						ValueExpr: `concat("Bearer ", $channel.key)`,
					},
				},
			},
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider: "openai",
		File:     pf,
		Meta: &dslmeta.Meta{
			API: "chat.completions",
		},
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result == nil {
		t.Fatalf("Query returned nil result")
	}
	if len(result.IDs) != 2 || result.IDs[0] != "gpt-4o-mini" || result.IDs[1] != "gpt-4.1" {
		t.Fatalf("ids=%v", result.IDs)
	}
}

func TestQuery_UsesInjectedHTTPClient(t *testing.T) {
	fakeClient := httpclienttest.NewFakeDoer(t,
		httpclienttest.NewStringResponse(http.StatusOK, `{"data":[{"id":"fake-model"}]}`),
	)

	pf := dslconfig.ProviderFile{
		Headers: dslconfig.ProviderHeaders{
			Defaults: dslconfig.PhaseHeaders{
				Auth: []dslconfig.HeaderOp{
					{
						Op:        "header_set",
						NameExpr:  `"Authorization"`,
						ValueExpr: `concat("Bearer ", $channel.key)`,
					},
				},
			},
		},
		Models: dslconfig.ProviderModels{
			Defaults: dslconfig.ModelsQueryConfig{
				Mode:    "custom",
				Method:  "GET",
				Path:    "/v1/models",
				IDPaths: []string{"$.data[*].id"},
			},
		},
	}

	result, err := Query(context.Background(), Params{
		Provider:   "openai",
		File:       pf,
		Meta:       &dslmeta.Meta{API: "chat.completions"},
		APIKey:     "sk-test",
		BaseURL:    "https://example.test",
		HTTPClient: fakeClient,
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if result == nil {
		t.Fatalf("Query returned nil result")
	}
	if len(result.IDs) != 1 || result.IDs[0] != "fake-model" {
		t.Fatalf("ids=%v", result.IDs)
	}
	reqs := fakeClient.Requests()
	if len(reqs) != 1 {
		t.Fatalf("requests=%d", len(reqs))
	}
	if reqs[0].URL.String() != "https://example.test/v1/models" {
		t.Fatalf("unexpected request url: %s", reqs[0].URL.String())
	}
	if reqs[0].Header.Get("Authorization") != "Bearer sk-test" {
		t.Fatalf("unexpected Authorization header: %s", reqs[0].Header.Get("Authorization"))
	}
}
