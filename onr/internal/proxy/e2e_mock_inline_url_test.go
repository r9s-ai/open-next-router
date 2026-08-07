package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// providerConfInlineURL routes a JSON API whose upstream answers with a link,
// and inlines that link only when the caller asked for inline content.
func providerConfInlineURL(baseURL string) string {
	return fmt.Sprintf(`syntax "next-router/0.1";

provider "linky" {
  defaults {
    upstream_config {
      base_url = %q;
    }
    auth {
      auth_bearer;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "images.generations" {
    upstream {
      set_path "/generate";
    }
    response {
      resp_inline_url path="$.data[*].url" set="b64_json"
                      when_request="$.response_format" when_eq="b64_json"
                      timeout_ms=5000 concurrency=2;
    }
  }
}
`, baseURL)
}

// newInlineURLClient returns a client whose HTTP timeout tolerates the asset
// fetch as well as the upstream call.
func newInlineURLClient(t *testing.T, conf string) *Client {
	t.Helper()
	c := newMockE2EClient(t, map[string]string{"linky.conf": conf})
	c.HTTP = &http.Client{Timeout: 10 * time.Second}
	return c
}

func TestE2EMock_InlineURL_FetchesOnlyWhenAsked(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const asset = "IMAGE-BYTES"
	var assetHits int
	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assetHits++
		_, _ = w.Write([]byte(asset))
	}))
	t.Cleanup(assets.Close)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"url":%q}]}`, assets.URL+"/a.png")
	}))
	t.Cleanup(upstream.Close)

	conf := providerConfInlineURL(upstream.URL)

	// response_format=url: the link must be handed through untouched and the
	// asset must not be fetched at all.
	c := newInlineURLClient(t, conf)
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(`{"model":"m","prompt":"p","response_format":"url"}`))
	if _, err := c.ProxyJSON(gc, "linky", ProviderKey{Name: "k", Value: "v"}, "images.generations", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	item := firstDataItem(t, out)
	if item["url"] == nil {
		t.Fatalf("url should pass through: %s", rec.Body.String())
	}
	if assetHits != 0 {
		t.Fatalf("asset fetched %d times for response_format=url", assetHits)
	}

	// response_format=b64_json: the asset is fetched and replaces the link.
	c = newInlineURLClient(t, conf)
	gc, rec = newGinJSONRequestPath(t, "/v1/images/generations", []byte(`{"model":"m","prompt":"p","response_format":"b64_json"}`))
	if _, err := c.ProxyJSON(gc, "linky", ProviderKey{Name: "k", Value: "v"}, "images.generations", false); err != nil {
		t.Fatalf("proxy error: %v", err)
	}
	out = map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	item = firstDataItem(t, out)
	if item["b64_json"] != base64.StdEncoding.EncodeToString([]byte(asset)) {
		t.Fatalf("b64_json=%v body=%s", item["b64_json"], rec.Body.String())
	}
	if _, ok := item["url"]; ok {
		t.Fatalf("url should be replaced: %s", rec.Body.String())
	}
	if assetHits != 1 {
		t.Fatalf("asset fetched %d times, want 1", assetHits)
	}
}

// A failing asset host must not fail the response: the caller still receives
// the link the upstream returned.
func TestE2EMock_InlineURL_FetchFailureKeepsTheLink(t *testing.T) {
	gin.SetMode(gin.TestMode)

	assets := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	t.Cleanup(assets.Close)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"created":1,"data":[{"url":%q}]}`, assets.URL+"/missing.png")
	}))
	t.Cleanup(upstream.Close)

	c := newInlineURLClient(t, providerConfInlineURL(upstream.URL))
	gc, rec := newGinJSONRequestPath(t, "/v1/images/generations", []byte(`{"model":"m","prompt":"p","response_format":"b64_json"}`))

	res, err := c.ProxyJSON(gc, "linky", ProviderKey{Name: "k", Value: "v"}, "images.generations", false)
	if err != nil {
		t.Fatalf("a failed asset fetch must not fail the response: %v", err)
	}
	if res == nil || res.Status != http.StatusOK {
		t.Fatalf("unexpected result: %#v", res)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	item := firstDataItem(t, out)
	if item["url"] == nil {
		t.Fatalf("link must survive a failed fetch: %s", rec.Body.String())
	}
	if _, ok := item["b64_json"]; ok {
		t.Fatalf("no inline content should be invented: %s", rec.Body.String())
	}
}

func firstDataItem(t *testing.T, out map[string]any) map[string]any {
	t.Helper()
	data, _ := out["data"].([]any)
	if len(data) == 0 {
		t.Fatalf("response has no data items: %#v", out)
	}
	item, _ := data[0].(map[string]any)
	if item == nil {
		t.Fatalf("data[0] is not an object: %#v", data[0])
	}
	return item
}
