package apitypes

import "testing"

// Keep compile-time coverage for req_map/resp_map typed conversion contracts.
func TestReqRespMapTypesImplementToMapper(t *testing.T) {
	var _ ToMapper = (*OpenAIChatCompletionsRequest)(nil)
	var _ ToMapper = (*OpenAIResponsesRequest)(nil)
	var _ ToMapper = (*ClaudeRequest)(nil)
	var _ ToMapper = (*ChatRequest)(nil) // GeminiGenerateContentRequest alias

	var _ ToMapper = (*OpenAIChatCompletionsResponse)(nil)
	var _ ToMapper = (*OpenAIResponsesResponse)(nil)
	var _ ToMapper = (*ClaudeResponse)(nil)
	var _ ToMapper = (*ClaudeUsage)(nil)
	var _ ToMapper = (*OpenAIChatCompletionsUsage)(nil)
	var _ ToMapper = (*OpenAIResponsesUsage)(nil)
	var _ ToMapper = (*UsageMetadata)(nil)
	var _ ToMapper = (*Part)(nil)
}
