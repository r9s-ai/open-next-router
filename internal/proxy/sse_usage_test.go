package proxy

import "testing"

func TestExtractLastSSEJSONWithUsage_GeminiUsageMetadata(t *testing.T) {
	sse := []byte("" +
		"data: {\"candidates\":[]}\n\n" +
		"data: {\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":2,\"thoughtsTokenCount\":3,\"totalTokenCount\":6}}\n\n")
	got := extractLastSSEJSONWithUsage(sse)
	if len(got) == 0 {
		t.Fatalf("got empty")
	}
	wantSub := "\"usageMetadata\""
	if string(got) == "" || !contains(string(got), wantSub) {
		t.Fatalf("got=%s, want contain %s", string(got), wantSub)
	}
}

func TestExtractLastSSEJSONWithUsage_OpenAIUsage(t *testing.T) {
	sse := []byte("" +
		"data: {\"choices\":[]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2,\"total_tokens\":3}}\n\n")
	got := extractLastSSEJSONWithUsage(sse)
	if len(got) == 0 {
		t.Fatalf("got empty")
	}
	wantSub := "\"usage\""
	if !contains(string(got), wantSub) {
		t.Fatalf("got=%s, want contain %s", string(got), wantSub)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	// tiny helper to avoid importing strings
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
