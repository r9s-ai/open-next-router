package dslmeta

import (
	"net/http"
	"testing"
	"time"
)

func TestMetaRequestRootJSONCached(t *testing.T) {
	meta := &Meta{
		RequestContentType: "application/json",
		RequestBody:        []byte(`{"model":"gpt-4o-mini","n":3}`),
	}

	root1 := meta.RequestRoot()
	if root1 == nil {
		t.Fatalf("expected request root")
	}
	if got, want := root1["model"], "gpt-4o-mini"; got != want {
		t.Fatalf("model got %v, want %v", got, want)
	}
	if got, want := root1["n"], float64(3); got != want {
		t.Fatalf("n got %v, want %v", got, want)
	}

	root1["cached"] = "yes"
	root2 := meta.RequestRoot()
	if root2 == nil {
		t.Fatalf("expected cached request root")
	}
	if got, want := root2["cached"], "yes"; got != want {
		t.Fatalf("cached marker got %v, want %v", got, want)
	}
}

func TestMetaRequestRootMultipartCached(t *testing.T) {
	meta := &Meta{
		RequestContentType: "multipart/form-data; boundary=abc123",
		RequestBody: []byte(
			"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"prompt\"\r\n\r\n" +
				"draw a cat\r\n" +
				"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"tag\"\r\n\r\n" +
				"a\r\n" +
				"--abc123\r\n" +
				"Content-Disposition: form-data; name=\"tag\"\r\n\r\n" +
				"b\r\n" +
				"--abc123--\r\n",
		),
	}

	root1 := meta.RequestRoot()
	if root1 == nil {
		t.Fatalf("expected multipart request root")
	}
	if got, want := root1["prompt"], "draw a cat"; got != want {
		t.Fatalf("prompt got %v, want %v", got, want)
	}
	tags, ok := root1["tag"].([]any)
	if !ok {
		t.Fatalf("tag type=%T, want []any", root1["tag"])
	}
	if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("tags got %v, want [a b]", tags)
	}

	root1["cached"] = "yes"
	root2 := meta.RequestRoot()
	if got, want := root2["cached"], "yes"; got != want {
		t.Fatalf("cached marker got %v, want %v", got, want)
	}
}

func TestCloneNil(t *testing.T) {
	got := Clone(nil)
	if got == nil {
		t.Fatalf("Clone(nil) returned nil")
	}
	if got.API != "" || got.RequestHeaders != nil || got.DerivedUsage != nil {
		t.Fatalf("Clone(nil)=%#v", got)
	}
}

func TestCloneCopiesPublicFields(t *testing.T) {
	start := time.Unix(123, 0)
	body := []byte(`{"model":"gpt-4o-mini"}`)
	src := &Meta{
		API:                 "chat.completions",
		IsStream:            true,
		BaseURL:             "https://example.test",
		APIKey:              "sk-test",
		OAuthAccessToken:    "oauth",
		OAuthCacheKey:       "cache",
		CredentialFile:      "/tmp/cred.json",
		CredentialJSON:      "{}",
		CredentialProjectID: "project",
		ChannelLocation:     "us-central1",
		OriginModelName:     "gpt-4o",
		DSLModelMapped:      "gpt-4o-mini",
		RequestURLPath:      "/v1/chat/completions",
		RequestContentType:  "application/json",
		RequestBody:         body,
		RequestHeaders:      http.Header{"X-Test": {"a"}},
		DerivedUsage:        map[string]any{"audio_duration_seconds": 1.5},
		StartTime:           start,
	}

	got := Clone(src)
	if got == nil {
		t.Fatalf("Clone returned nil")
	}
	if got.API != src.API || !got.IsStream || got.BaseURL != src.BaseURL ||
		got.APIKey != src.APIKey || got.OAuthAccessToken != src.OAuthAccessToken ||
		got.OAuthCacheKey != src.OAuthCacheKey || got.CredentialFile != src.CredentialFile ||
		got.CredentialJSON != src.CredentialJSON || got.CredentialProjectID != src.CredentialProjectID ||
		got.ChannelLocation != src.ChannelLocation || got.OriginModelName != src.OriginModelName ||
		got.DSLModelMapped != src.DSLModelMapped || got.RequestURLPath != src.RequestURLPath ||
		got.RequestContentType != src.RequestContentType || !got.StartTime.Equal(start) {
		t.Fatalf("Clone fields differ: %#v", got)
	}
	if len(got.RequestBody) != len(body) || &got.RequestBody[0] != &body[0] {
		t.Fatalf("RequestBody should reuse source slice")
	}
}

func TestCloneCopiesHeaderAndDerivedUsageContainers(t *testing.T) {
	src := &Meta{
		RequestHeaders: http.Header{"X-Test": {"a"}},
		DerivedUsage:   map[string]any{"n": 1},
	}

	got := Clone(src)
	got.RequestHeaders.Set("X-Test", "b")
	got.DerivedUsage["n"] = 2

	if src.RequestHeaders.Get("X-Test") != "a" {
		t.Fatalf("source header mutated: %v", src.RequestHeaders)
	}
	if src.DerivedUsage["n"] != 1 {
		t.Fatalf("source derived usage mutated: %v", src.DerivedUsage)
	}
}
