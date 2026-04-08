package apitransform

import (
	"fmt"
	"io"
	"strings"
)

// NormalizeSSETransformMode canonicalizes sse_parse mode names so callers can
// share one supported-mode registry across runtimes.
func NormalizeSSETransformMode(mode string) string {
	return strings.ToLower(strings.TrimSpace(mode))
}

// SupportsSSETransformMode reports whether onr-core has a shared transform for
// the given sse_parse mode.
func SupportsSSETransformMode(mode string) bool {
	switch NormalizeSSETransformMode(mode) {
	case "openai_responses_to_openai_chat_chunks",
		"anthropic_to_openai_chunks",
		"openai_to_anthropic_chunks",
		"openai_to_gemini_chunks",
		"gemini_to_openai_chat_chunks":
		return true
	default:
		return false
	}
}

// TransformSSEByMode runs the shared sse_parse transform selected by mode.
func TransformSSEByMode(mode string, src io.Reader, dst io.Writer) error {
	switch NormalizeSSETransformMode(mode) {
	case "openai_responses_to_openai_chat_chunks":
		return TransformOpenAIResponsesSSEToChatCompletionsSSE(src, dst)
	case "anthropic_to_openai_chunks":
		return TransformClaudeMessagesSSEToOpenAIChatCompletionsSSE(src, dst)
	case "openai_to_anthropic_chunks":
		return TransformOpenAIChatCompletionsSSEToClaudeMessagesSSE(src, dst)
	case "openai_to_gemini_chunks":
		return TransformOpenAIChatCompletionsSSEToGeminiSSE(src, dst)
	case "gemini_to_openai_chat_chunks":
		return TransformGeminiSSEToOpenAIChatCompletionsSSE(src, dst)
	default:
		return fmt.Errorf("unsupported sse_parse mode %q", strings.TrimSpace(mode))
	}
}
