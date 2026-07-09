package proxy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/requestvalidate"
)

func providerConfWithValidation(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "validated" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    request {
      req_required body "$.model";
    }
    response {
      resp_passthrough;
    }
  }

  match api = "chat.completions" {
    request {
      req_required body "$.messages";
      req_type body "$.messages" array;
      req_len body "$.messages" min=1;
      req_type body "$.temperature" number;
      req_range body "$.temperature" min=0 max=2;
      req_enum body "$.reasoning_effort" "low" "medium" "high";
      req_forbid body "$.legacy_field";
    }
    upstream {
      set_path "/v1/chat/completions";
    }
  }
}
`, baseURL)
}

func newValidationE2E(t *testing.T) (*Client, *atomic.Int64) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var upstreamCalls atomic.Int64
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"ok","choices":[]}`))
	}))
	t.Cleanup(mock.Close)

	c := newMockE2EClient(t, map[string]string{
		"validated.conf": providerConfWithValidation(mock.URL),
	})
	return c, &upstreamCalls
}

func TestE2EMock_RequestValidation_FailureSkipsUpstream(t *testing.T) {
	c, upstreamCalls := newValidationE2E(t)

	cases := []struct {
		name      string
		body      string
		wantParam string
	}{
		{"missing messages", `{"model":"m1"}`, "$.messages"},
		{"messages not array", `{"model":"m1","messages":"hi"}`, "$.messages"},
		{"messages empty", `{"model":"m1","messages":[]}`, "$.messages"},
		{"temperature out of range", `{"model":"m1","messages":[{"role":"user","content":"x"}],"temperature":3}`, "$.temperature"},
		{"bad enum", `{"model":"m1","messages":[{"role":"user","content":"x"}],"reasoning_effort":"max"}`, "$.reasoning_effort"},
		{"forbidden field", `{"model":"m1","messages":[{"role":"user","content":"x"}],"legacy_field":1}`, "$.legacy_field"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gc, _ := newGinJSONRequest(t, []byte(tc.body))
			_, err := c.ProxyJSON(gc, "validated", ProviderKey{Name: "k", Value: "v"}, "chat.completions", false)
			var verr *requestvalidate.RequestValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected *RequestValidationError, got %T: %v", err, err)
			}
			if verr.PathOrName != tc.wantParam {
				t.Fatalf("unexpected param: got %q want %q", verr.PathOrName, tc.wantParam)
			}
		})
	}
	if n := upstreamCalls.Load(); n != 0 {
		t.Fatalf("mock upstream received %d requests, expected 0", n)
	}
}

func TestE2EMock_RequestValidation_PassReachesUpstream(t *testing.T) {
	c, upstreamCalls := newValidationE2E(t)

	body := `{"model":"m1","messages":[{"role":"user","content":"x"}],"temperature":0.7,"reasoning_effort":"low"}`
	gc, rec := newGinJSONRequest(t, []byte(body))
	res, err := c.ProxyJSON(gc, "validated", ProviderKey{Name: "k", Value: "v"}, "chat.completions", false)
	if err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("unexpected status: %d", got)
	}
	if n := upstreamCalls.Load(); n != 1 {
		t.Fatalf("mock upstream received %d requests, expected 1", n)
	}
}
