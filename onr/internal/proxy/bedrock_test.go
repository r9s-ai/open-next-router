package proxy

import "testing"

func TestBedrockRuntimeTargetHTTPPassthroughPaths(t *testing.T) {
	for _, path := range []string{
		"/v1/chat/completions",
		"/openai/v1/chat/completions",
		"/v1/responses",
		"/anthropic/v1/messages",
		"/custom/v1/anything",
	} {
		t.Run(path, func(t *testing.T) {
			op, _, err := bedrockRuntimeTarget(path)
			if err != nil {
				t.Fatalf("bedrockRuntimeTarget: %v", err)
			}
			if op != "http-passthrough" {
				t.Fatalf("operation=%q", op)
			}
		})
	}
}
