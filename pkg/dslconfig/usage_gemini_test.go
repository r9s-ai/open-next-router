package dslconfig

import (
	"testing"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
)

func TestExtractUsage_Gemini_NonStream(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent", IsStream: false}
	cfg := UsageExtractConfig{Mode: "gemini"}

	resp := []byte(`{
	  "candidates":[{"content":{"parts":[{"text":"hi"}]}}],
	  "usageMetadata":{
	    "promptTokenCount": 11,
	    "candidatesTokenCount": 9,
	    "thoughtsTokenCount": 3,
	    "totalTokenCount": 23
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 11 {
		t.Fatalf("input_tokens=%d want=11", u.InputTokens)
	}
	// new-api alignment: completion = candidates + thoughts
	if u.OutputTokens != 12 {
		t.Fatalf("output_tokens=%d want=12", u.OutputTokens)
	}
	if u.TotalTokens != 23 {
		t.Fatalf("total_tokens=%d want=23", u.TotalTokens)
	}
}
