// Package respinline implements the resp_inline_url response rule: fetch the
// URLs a mapped response points at and inline their content as base64.
//
// It is the one part of the response pipeline that performs network I/O, so the
// rules it follows are deliberately narrow:
//
//   - it only ever issues GET requests to http/https URLs taken from the
//     upstream response;
//   - every fetch is bounded by the rule's timeout, size limit and concurrency;
//   - a failed fetch leaves the URL in place rather than failing the response,
//     because a caller holding a link can still retrieve the asset while a
//     failed request gives them nothing.
package respinline

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// Result reports what Apply did, for logging and metrics. Failures are counted
// rather than returned: they are per-URL and non-fatal by design.
type Result struct {
	Attempted int
	Inlined   int
	Failed    int
	// FirstError is the first fetch failure, kept so callers can log one
	// representative cause instead of every URL's.
	FirstError error
}

// Apply fetches the URLs rule.Path addresses in root and replaces each with its
// base64 content under rule.SetField. root is mutated in place.
//
// requestRoot is the client's request object, used only to evaluate the rule's
// gate; a nil requestRoot means an ungated rule still runs and a gated one does
// not. doer must be non-nil when the rule is expected to run.
func Apply(
	ctx context.Context,
	root map[string]any,
	requestRoot map[string]any,
	rule *dslconfig.RespInlineURLRule,
	doer httpclient.HTTPDoer,
) Result {
	if rule == nil || root == nil || doer == nil {
		return Result{}
	}
	if !gateAllows(requestRoot, rule) {
		return Result{}
	}

	targets := collectTargets(root, rule.Path)
	if len(targets) == 0 {
		return Result{}
	}

	concurrency := rule.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > len(targets) {
		concurrency = len(targets)
	}

	var (
		mu     sync.Mutex
		result = Result{Attempted: len(targets)}
	)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := range targets {
		wg.Add(1)
		go func(t *target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			encoded, err := fetchBase64(ctx, doer, t.url, rule)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				result.Failed++
				if result.FirstError == nil {
					result.FirstError = err
				}
				return
			}
			// Replace only on success, so a failure leaves a usable link.
			t.owner[rule.SetField] = encoded
			delete(t.owner, t.field)
			result.Inlined++
		}(&targets[i])
	}
	wg.Wait()
	return result
}

// gateAllows reports whether the rule's when_request condition is satisfied.
func gateAllows(requestRoot map[string]any, rule *dslconfig.RespInlineURLRule) bool {
	path := strings.TrimSpace(rule.WhenRequestPath)
	if path == "" {
		return true
	}
	if requestRoot == nil {
		return false
	}
	got := strings.TrimSpace(jsonutil.GetStringByPath(requestRoot, path))
	return strings.EqualFold(got, strings.TrimSpace(rule.WhenEquals))
}

// target is one URL-bearing field: the object holding it, the field name, and
// the URL itself. Keeping the owner lets Apply swap the field in place.
type target struct {
	owner map[string]any
	field string
	url   string
}

// collectTargets resolves the rule path to the objects and field names holding
// URLs. It supports the "$.a[*].b" shape the rule is written against; anything
// else yields nothing rather than guessing.
func collectTargets(root map[string]any, path string) []target {
	arrayPath, field, ok := splitTrailingField(path)
	if !ok {
		return nil
	}
	values, ok := jsonutil.GetValuesByPath(root, arrayPath)
	if !ok {
		return nil
	}
	out := make([]target, 0, len(values))
	for _, value := range values {
		owner, ok := value.(map[string]any)
		if !ok {
			continue
		}
		raw := strings.TrimSpace(jsonutil.CoerceString(owner[field]))
		if raw == "" {
			continue
		}
		out = append(out, target{owner: owner, field: field, url: raw})
	}
	return out
}

// splitTrailingField splits "$.data[*].url" into "$.data[*]" and "url".
func splitTrailingField(path string) (string, string, bool) {
	trimmed := strings.TrimSpace(path)
	idx := strings.LastIndex(trimmed, ".")
	if idx <= 0 || idx == len(trimmed)-1 {
		return "", "", false
	}
	field := trimmed[idx+1:]
	if field == "" || strings.ContainsAny(field, "[]*$") {
		return "", "", false
	}
	return trimmed[:idx], field, true
}

// fetchBase64 downloads one URL and returns its base64 content.
func fetchBase64(ctx context.Context, doer httpclient.HTTPDoer, rawURL string, rule *dslconfig.RespInlineURLRule) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	// The URL comes from an upstream response, so restrict what a compromised
	// or buggy provider can make this process request.
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}

	fetchCtx := ctx
	if rule.TimeoutMS > 0 {
		var cancel context.CancelFunc
		fetchCtx, cancel = context.WithTimeout(ctx, durationMS(rule.TimeoutMS))
		defer cancel()
	}
	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	resp, err := doFetch(doer, req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch returned status %d", resp.StatusCode)
	}

	// Read one byte past the limit so an oversized body is rejected rather than
	// silently truncated into a corrupt asset.
	limit := rule.MaxBytes
	if limit <= 0 {
		limit = respInlineDefaultMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	if int64(len(body)) > limit {
		return "", fmt.Errorf("body exceeds max_bytes (%d)", limit)
	}
	return base64.StdEncoding.EncodeToString(body), nil
}

const respInlineDefaultMaxBytes = 10 << 20

// durationMS converts milliseconds to a time.Duration.
func durationMS(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

// doFetch isolates the caller's HTTP client. A caller can hand over a nil
// *http.Client held in a non-nil interface, which the nil check in Apply cannot
// see; the resulting panic would happen on a fetch goroutine and take the
// process down rather than degrading to the URL the rule promises to keep.
func doFetch(doer httpclient.HTTPDoer, req *http.Request) (resp *http.Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			resp, err = nil, fmt.Errorf("fetch panicked: %v", r)
		}
	}()
	resp, err = doer.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	if resp == nil {
		return nil, fmt.Errorf("fetch returned no response")
	}
	return resp, nil
}
