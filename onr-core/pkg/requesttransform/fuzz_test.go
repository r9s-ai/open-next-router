package requesttransform

import (
	"encoding/json"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func FuzzApply(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`),
		[]byte(`{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`),
		[]byte(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`),
		[]byte(`{}`),
	} {
		f.Add(seed, uint8(0))
	}

	modes := []string{
		"",
		"openai_chat_to_openai_responses",
		"openai_chat_to_anthropic_messages",
		"openai_chat_to_gemini_generate_content",
		"anthropic_to_openai_chat",
		"gemini_to_openai_chat",
	}
	f.Fuzz(func(t *testing.T, body []byte, modeIndex uint8) {
		var value map[string]any
		if err := json.Unmarshal(body, &value); err != nil {
			return
		}

		transform := &dslconfig.RequestTransform{
			ReqMapMode: modes[int(modeIndex)%len(modes)],
			JSONOps: []dslconfig.JSONOp{
				{Op: "json_set", Path: "$.metadata.fuzz", ValueExpr: `true`},
			},
		}
		result, err := Apply(&dslmeta.Meta{}, "application/json", body, value, transform, ApplyOptions{})
		if err != nil {
			return
		}
		if result.Root == nil {
			t.Fatal("successful transform returned a nil root")
		}
		var output map[string]any
		if err := json.Unmarshal(result.Body, &output); err != nil {
			t.Fatalf("successful transform returned invalid JSON: %v", err)
		}
		if output == nil {
			t.Fatal("successful transform returned a non-object JSON body")
		}
	})
}
