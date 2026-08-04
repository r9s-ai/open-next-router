package respinline

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

// fakeDoer serves canned bodies per URL and records what was requested.
type fakeDoer struct {
	bodies map[string][]byte
	status map[string]int
	err    map[string]error

	mu       sync.Mutex
	requests []string
	inFlight int32
	maxPar   int32
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	cur := atomic.AddInt32(&f.inFlight, 1)
	for {
		peak := atomic.LoadInt32(&f.maxPar)
		if cur <= peak || atomic.CompareAndSwapInt32(&f.maxPar, peak, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inFlight, -1)

	url := req.URL.String()
	f.mu.Lock()
	f.requests = append(f.requests, url)
	f.mu.Unlock()

	if err, ok := f.err[url]; ok {
		return nil, err
	}
	status := http.StatusOK
	if s, ok := f.status[url]; ok {
		status = s
	}
	body := f.bodies[url]
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(string(body))),
		Header:     http.Header{},
	}, nil
}

func baseRule() *dslconfig.RespInlineURLRule {
	return &dslconfig.RespInlineURLRule{
		Path:        "$.data[*].url",
		SetField:    "b64_json",
		MaxBytes:    1 << 20,
		TimeoutMS:   1000,
		Concurrency: 4,
	}
}

func rootWithURLs(urls ...string) map[string]any {
	items := make([]any, 0, len(urls))
	for _, u := range urls {
		items = append(items, map[string]any{"url": u})
	}
	return map[string]any{"created": 1, "data": items}
}

func itemAt(t *testing.T, root map[string]any, i int) map[string]any {
	t.Helper()
	data, _ := root["data"].([]any)
	if i >= len(data) {
		t.Fatalf("data has %d items, wanted index %d", len(data), i)
	}
	item, _ := data[i].(map[string]any)
	if item == nil {
		t.Fatalf("data[%d] is not an object: %#v", i, data[i])
	}
	return item
}

func TestApply_InlinesAndDropsTheURL(t *testing.T) {
	const u = "https://example.invalid/a.png"
	doer := &fakeDoer{bodies: map[string][]byte{u: []byte("PNGDATA")}}
	root := rootWithURLs(u)

	res := Apply(context.Background(), root, nil, baseRule(), doer)
	if res.Inlined != 1 || res.Failed != 0 {
		t.Fatalf("result=%+v want 1 inlined", res)
	}
	item := itemAt(t, root, 0)
	if item["b64_json"] != base64.StdEncoding.EncodeToString([]byte("PNGDATA")) {
		t.Fatalf("b64_json=%v", item["b64_json"])
	}
	// The URL must be gone, otherwise the caller sees both forms and cannot
	// tell which one the response format promised.
	if _, ok := item["url"]; ok {
		t.Fatalf("url should be removed after inlining: %#v", item)
	}
}

// A failed fetch has to leave a usable link: a caller holding a URL can still
// retrieve the asset, a caller holding nothing cannot.
func TestApply_FailureKeepsTheURL(t *testing.T) {
	const ok1 = "https://example.invalid/ok.png"
	const bad = "https://example.invalid/bad.png"
	doer := &fakeDoer{
		bodies: map[string][]byte{ok1: []byte("A")},
		err:    map[string]error{bad: fmt.Errorf("boom")},
	}
	root := rootWithURLs(ok1, bad)

	res := Apply(context.Background(), root, nil, baseRule(), doer)
	if res.Attempted != 2 || res.Inlined != 1 || res.Failed != 1 {
		t.Fatalf("result=%+v", res)
	}
	if res.FirstError == nil {
		t.Fatal("expected FirstError to be reported")
	}
	if _, ok := itemAt(t, root, 0)["url"]; ok {
		t.Fatal("successful item should have dropped its url")
	}
	failed := itemAt(t, root, 1)
	if failed["url"] != bad {
		t.Fatalf("failed item lost its url: %#v", failed)
	}
	if _, ok := failed["b64_json"]; ok {
		t.Fatalf("failed item must not carry b64_json: %#v", failed)
	}
}

func TestApply_NonOKStatusIsAFailure(t *testing.T) {
	const u = "https://example.invalid/a.png"
	doer := &fakeDoer{bodies: map[string][]byte{u: []byte("nope")}, status: map[string]int{u: 404}}
	root := rootWithURLs(u)

	if res := Apply(context.Background(), root, nil, baseRule(), doer); res.Failed != 1 {
		t.Fatalf("result=%+v want 1 failed", res)
	}
	if itemAt(t, root, 0)["url"] != u {
		t.Fatal("url should survive a 404")
	}
}

// An oversized body is rejected rather than truncated: half an image is worse
// than a link to the whole one.
func TestApply_OversizedBodyIsRejectedNotTruncated(t *testing.T) {
	const u = "https://example.invalid/big.png"
	doer := &fakeDoer{bodies: map[string][]byte{u: []byte("0123456789")}}
	rule := baseRule()
	rule.MaxBytes = 4
	root := rootWithURLs(u)

	res := Apply(context.Background(), root, nil, rule, doer)
	if res.Failed != 1 {
		t.Fatalf("result=%+v want 1 failed", res)
	}
	if _, ok := itemAt(t, root, 0)["b64_json"]; ok {
		t.Fatal("a truncated body must not be inlined")
	}
}

// The URL comes from an upstream response, so a non-http scheme must not be
// fetched at all.
func TestApply_RejectsNonHTTPSchemes(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "ftp://example.invalid/a.png", "gopher://x"} {
		doer := &fakeDoer{bodies: map[string][]byte{}}
		root := rootWithURLs(u)
		res := Apply(context.Background(), root, nil, baseRule(), doer)
		if res.Failed != 1 {
			t.Fatalf("%s: result=%+v want 1 failed", u, res)
		}
		if len(doer.requests) != 0 {
			t.Fatalf("%s: must not be requested, got %v", u, doer.requests)
		}
	}
}

func TestApply_GateSkipsWhenRequestFieldDiffers(t *testing.T) {
	const u = "https://example.invalid/a.png"
	rule := baseRule()
	rule.WhenRequestPath = "$.response_format"
	rule.WhenEquals = "b64_json"

	doer := &fakeDoer{bodies: map[string][]byte{u: []byte("A")}}
	root := rootWithURLs(u)
	if res := Apply(context.Background(), root, map[string]any{"response_format": "url"}, rule, doer); res.Attempted != 0 {
		t.Fatalf("result=%+v want no attempt for response_format=url", res)
	}
	if len(doer.requests) != 0 {
		t.Fatalf("gate should prevent any fetch, got %v", doer.requests)
	}

	root = rootWithURLs(u)
	if res := Apply(context.Background(), root, map[string]any{"response_format": "b64_json"}, rule, doer); res.Inlined != 1 {
		t.Fatalf("result=%+v want 1 inlined for response_format=b64_json", res)
	}
}

// A gated rule with no request object must not fetch: the gate cannot be shown
// to hold, and fetching is the expensive, surprising direction.
func TestApply_GatedRuleWithoutRequestRootDoesNothing(t *testing.T) {
	const u = "https://example.invalid/a.png"
	rule := baseRule()
	rule.WhenRequestPath = "$.response_format"
	rule.WhenEquals = "b64_json"
	doer := &fakeDoer{bodies: map[string][]byte{u: []byte("A")}}

	if res := Apply(context.Background(), rootWithURLs(u), nil, rule, doer); res.Attempted != 0 {
		t.Fatalf("result=%+v want no attempt", res)
	}
}

func TestApply_ConcurrencyIsCapped(t *testing.T) {
	urls := make([]string, 0, 8)
	bodies := map[string][]byte{}
	for i := 0; i < 8; i++ {
		u := fmt.Sprintf("https://example.invalid/%d.png", i)
		urls = append(urls, u)
		bodies[u] = []byte("x")
	}
	doer := &fakeDoer{bodies: bodies}
	rule := baseRule()
	rule.Concurrency = 2

	if res := Apply(context.Background(), rootWithURLs(urls...), nil, rule, doer); res.Inlined != 8 {
		t.Fatalf("result=%+v want 8 inlined", res)
	}
	if peak := atomic.LoadInt32(&doer.maxPar); peak > 2 {
		t.Fatalf("peak parallelism=%d want <= 2", peak)
	}
}

func TestApply_NoOpCases(t *testing.T) {
	doer := &fakeDoer{bodies: map[string][]byte{}}
	cases := map[string]struct {
		root map[string]any
		rule *dslconfig.RespInlineURLRule
		doer *fakeDoer
	}{
		"nil rule":           {rootWithURLs("https://x/a"), nil, doer},
		"nil root":           {nil, baseRule(), doer},
		"nil doer":           {rootWithURLs("https://x/a"), baseRule(), nil},
		"no matching path":   {map[string]any{"other": 1}, baseRule(), doer},
		"empty url string":   {rootWithURLs("   "), baseRule(), doer},
		"data not an object": {map[string]any{"data": []any{"plain"}}, baseRule(), doer},
	}
	for name, tc := range cases {
		var d = tc.doer
		var res Result
		if d == nil {
			res = Apply(context.Background(), tc.root, nil, tc.rule, nil)
		} else {
			res = Apply(context.Background(), tc.root, nil, tc.rule, d)
		}
		if res.Attempted != 0 || res.Inlined != 0 {
			t.Fatalf("%s: result=%+v want no-op", name, res)
		}
	}
	if len(doer.requests) != 0 {
		t.Fatalf("no-op cases must not fetch, got %v", doer.requests)
	}
}
