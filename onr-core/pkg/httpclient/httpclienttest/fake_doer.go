package httpclienttest

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/httpclient"
)

// FakeDoer implements httpclient.HTTPDoer so callers can run tests without
// making outbound HTTP requests.
type FakeDoer struct {
	t         testing.TB
	responses []*http.Response
	requests  []*http.Request
}

// NewFakeDoer returns a FakeDoer seeded with the responses that should be
// returned for each Do call.
// NewFakeDoer returns a non-nil fake doer.
func NewFakeDoer(t testing.TB, responses ...*http.Response) *FakeDoer {
	return &FakeDoer{
		t:         t,
		responses: append([]*http.Response(nil), responses...),
	}
}

// Do records the request and returns the next queued response.
func (f *FakeDoer) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req)
	if len(f.responses) == 0 {
		f.t.Fatalf("fake http client has no responses left for request %s %s", req.Method, req.URL.String())
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

// Requests returns the HTTP requests captured so far.
func (f *FakeDoer) Requests() []*http.Request {
	return append([]*http.Request(nil), f.requests...)
}

// NewStringResponse builds a minimal http.Response with the provided status
// code and body string.
// NewStringResponse returns a non-nil HTTP response.
func NewStringResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

var _ httpclient.HTTPDoer = (*FakeDoer)(nil)
