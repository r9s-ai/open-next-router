package dslconfig

import (
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestExtractUsage_OpenAI_NonStream(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "input_tokens": 8,
	    "output_tokens": 9,
	    "input_tokens_details": {
	      "cached_tokens": 5
	    }
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 8 || u.OutputTokens != 9 || u.TotalTokens != 17 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 5 {
		t.Fatalf("cached=%d want=5", cached)
	}
	if u.InputTokenDetails == nil || u.InputTokenDetails.CachedTokens != 5 {
		t.Fatalf("expected cached token details, got=%+v", u.InputTokenDetails)
	}
}

func TestExtractUsage_OpenAI_ImagesGenerations(t *testing.T) {
	meta := &dslmeta.Meta{API: "images.generations", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "created": 1743264000,
	  "data": [{"b64_json":"abc"}],
	  "usage": {
	    "input_tokens": 104,
	    "input_tokens_details": {
	      "image_tokens": 0,
	      "text_tokens": 104
	    },
	    "output_tokens": 4096,
	    "output_tokens_details": {
	      "image_tokens": 4096,
	      "text_tokens": 0
	    },
	    "total_tokens": 4200
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 104 {
		t.Fatalf("input_tokens=%d want=104", u.InputTokens)
	}
	if u.OutputTokens != 4096 {
		t.Fatalf("output_tokens=%d want=4096", u.OutputTokens)
	}
	if u.TotalTokens != 4200 {
		t.Fatalf("total_tokens=%d want=4200", u.TotalTokens)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
	if got, want := u.FlatFields["image_generate_images"], 1; got != want {
		t.Fatalf("image_generate_images=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "image.generate" && fact.Unit == "image" && fact.Quantity == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected image.generate image fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_ImagesEditsCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{API: "images.edits", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "created": 1743264000,
	  "data": [{"b64_json":"abc"},{"b64_json":"def"}],
	  "usage": {
	    "input_tokens": 50,
	    "output_tokens": 80,
	    "total_tokens": 130
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.FlatFields["image_edit_images"], 2; got != want {
		t.Fatalf("image_edit_images=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "image.edit" && fact.Unit == "image" && fact.Quantity == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected image.edit image fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_AudioTranscriptionsCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{API: "audio.transcriptions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "text":"hello",
	  "usage": {
	    "type":"duration",
	    "seconds": 3
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.FlatFields["audio_stt_seconds"], 3; got != want {
		t.Fatalf("audio_stt_seconds=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "audio.stt" && fact.Unit == "second" && fact.Quantity == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected audio.stt second fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_AudioTranslationsCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{API: "audio.translations", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "text":"hello translated",
	  "usage": {
	    "type":"duration",
	    "seconds": 4
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.FlatFields["audio_translate_seconds"], 4; got != want {
		t.Fatalf("audio_translate_seconds=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "audio.translate" && fact.Unit == "second" && fact.Quantity == 4 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected audio.translate second fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_AudioSpeechDerivedCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{
		API:          "audio.speech",
		IsStream:     false,
		DerivedUsage: map[string]any{"audio_duration_seconds": 1.5},
	}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	u, _, err := ExtractUsage(meta, cfg, []byte("not-json-audio"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got := u.FlatFields["audio_tts_seconds"]; got == nil {
		t.Fatalf("audio_tts_seconds missing")
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "audio.tts" && fact.Unit == "second" && fact.Source == "derived" && fact.Quantity == 1.5 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected audio.tts derived fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_ImagesGenerationsMiniRealResponse(t *testing.T) {
	meta := &dslmeta.Meta{API: "images.generations", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", "images.generations", false)
	resp := mustReadSharedTestData(t, filepath.Join("openai", "images_generations_gpt_image_1_mini_real.json"))

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 13 || u.OutputTokens != 1056 || u.TotalTokens != 1069 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
	if got, want := u.FlatFields["image_generate_images"], 1; got != want {
		t.Fatalf("image_generate_images=%v want=%v", got, want)
	}
}

func TestExtractUsage_OpenAI_AudioTranscriptionsMiniRealResponse(t *testing.T) {
	meta := &dslmeta.Meta{API: "audio.transcriptions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", "audio.transcriptions", false)
	resp := mustReadSharedTestData(t, filepath.Join("openai", "audio_transcriptions_gpt_4o_mini_transcribe_real.json"))

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 10 || u.OutputTokens != 2 || u.TotalTokens != 12 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
}

func TestExtractUsage_OpenAI_AudioSpeechMiniRealResponse(t *testing.T) {
	meta := &dslmeta.Meta{
		API:          "audio.speech",
		IsStream:     false,
		DerivedUsage: map[string]any{"audio_duration_seconds": 2.352},
	}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", "audio.speech", false)
	resp := mustReadSharedTestData(t, filepath.Join("openai", "audio_speech_gpt_4o_mini_tts_real.mp3"))

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 {
		t.Fatalf("expected zero token usage, got %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
	if got, want := u.FlatFields["audio_tts_seconds"], 2.352; got != want {
		t.Fatalf("audio_tts_seconds=%v want=%v", got, want)
	}
}

func TestExtractUsage_OpenAI_ResponsesWebSearchCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "output": [
	    {"type":"web_search_call","status":"completed"},
	    {"type":"web_search_call","status":"failed"},
	    {"type":"message","status":"completed"},
	    {"type":"web_search_call","status":"completed"}
	  ]
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 2; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.Quantity == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected server_tool.web_search call fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_ResponsesWebSearchCanonicalFact_StreamFinalResponse(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: true}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "response": {
	    "output": [
	      {"type":"web_search_call","status":"completed"},
	      {"type":"web_search_call","status":"failed"},
	      {"type":"web_search_call","status":"completed"}
	    ]
	  }
	}`)

	root, err := responseRootFromBody(meta, *cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	u, _, err := extractUsageFromRootsWithEvent(meta, "response.completed", *cfg, nil, root, nil, resp)
	if err != nil {
		t.Fatalf("extractUsageFromRootsWithEvent: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 2; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.Quantity == 2 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected server_tool.web_search call fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_ChatCompletionsWebSearchCanonicalFact(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "choices": [
	    {
	      "message": {
	        "annotations": [
	          {"type": "url_citation"},
	          {"type": "url_citation"}
	        ]
	      }
	    }
	  ],
	  "usage": {
	    "prompt_tokens": 8,
	    "completion_tokens": 9
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.InputTokens, 8; got != want {
		t.Fatalf("input_tokens=%v want=%v", got, want)
	}
	if got, want := u.OutputTokens, 9; got != want {
		t.Fatalf("output_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.Quantity == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected server_tool.web_search call fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_OpenAI_ChatCompletionsMultimodalRealResponse(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := mustReadSharedTestData(t, filepath.Join("openai", "chat_completions_multimodal_real.json"))

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 36852 || u.OutputTokens != 1 || u.TotalTokens != 36853 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
}

func TestExtractUsage_OpenAI_ResponsesMultimodalRealResponse(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := mustReadSharedTestData(t, filepath.Join("openai", "responses_multimodal_real.json"))

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 36852 || u.OutputTokens != 2 || u.TotalTokens != 36854 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 0 {
		t.Fatalf("cached=%d want=0", cached)
	}
}

func TestExtractUsage_Anthropic_NonStream_TTLFactsAndProjection(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "input_tokens": 10,
	    "output_tokens": 170,
	    "cache_read_input_tokens": 4,
	    "cache_creation_input_tokens": 6802,
	    "cache_creation": {
	      "ephemeral_5m_input_tokens": 6802,
	      "ephemeral_1h_input_tokens": 0
	    }
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 6816 || u.OutputTokens != 170 || u.TotalTokens != 6986 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 4 {
		t.Fatalf("cached=%d want=4", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if u.InputTokenDetails.CachedTokens != 4 || u.InputTokenDetails.CacheWriteTokens != 6802 {
		t.Fatalf("unexpected InputTokenDetails: %+v", *u.InputTokenDetails)
	}
	if u.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := u.FlatFields["cache_write_ttl_5m_tokens"], 6802; got != want {
		t.Fatalf("cache_write_ttl_5m_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["cache_write_ttl_1h_tokens"], 0; got != want {
		t.Fatalf("cache_write_ttl_1h_tokens=%v want=%v", got, want)
	}
}

func TestExtractUsage_Anthropic_NonStream_CacheWriteFallbackOnly(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "cache_read_input_tokens": 3,
	    "cache_creation_input_tokens": 11
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.InputTokens, 14; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if cached != 3 {
		t.Fatalf("cached=%d want=3", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if u.InputTokenDetails.CachedTokens != 3 || u.InputTokenDetails.CacheWriteTokens != 11 {
		t.Fatalf("unexpected InputTokenDetails: %+v", *u.InputTokenDetails)
	}
}

func TestExtractUsage_Anthropic_NonStream_CacheReadOnly(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "cache_read_input_tokens": 6
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if got, want := u.InputTokens, 6; got != want {
		t.Fatalf("InputTokens got %d, want %d", got, want)
	}
	if cached != 6 {
		t.Fatalf("cached=%d want=6", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if u.InputTokenDetails.CachedTokens != 6 || u.InputTokenDetails.CacheWriteTokens != 0 {
		t.Fatalf("unexpected InputTokenDetails: %+v", *u.InputTokenDetails)
	}
}

func TestExtractUsage_AnthropicProvider_NonStream_WebSearchProjection(t *testing.T) {
	meta := &dslmeta.Meta{API: "claude.messages", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "anthropic.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "input_tokens": 105,
	    "output_tokens": 6039,
	    "cache_read_input_tokens": 7123,
	    "cache_creation_input_tokens": 7345,
	    "server_tool_use": {
	      "web_search_requests": 1
	    }
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 14573 || u.OutputTokens != 6039 || u.TotalTokens != 20612 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 7123 {
		t.Fatalf("cached=%d want=7123", cached)
	}
	if u.InputTokenDetails == nil {
		t.Fatalf("expected InputTokenDetails")
	}
	if u.InputTokenDetails.CachedTokens != 7123 || u.InputTokenDetails.CacheWriteTokens != 7345 {
		t.Fatalf("unexpected InputTokenDetails: %+v", *u.InputTokenDetails)
	}
	if u.FlatFields == nil {
		t.Fatalf("expected FlatFields")
	}
	if got, want := u.FlatFields["server_tool_web_search_calls"], 1; got != want {
		t.Fatalf("server_tool_web_search_calls=%v want=%v", got, want)
	}
	found := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "server_tool.web_search" && fact.Unit == "call" && fact.Quantity == 1 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected server_tool.web_search call fact, got=%#v", u.DebugFacts)
	}
}

func TestExtractUsage_Gemini_NonStream(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)

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

func TestExtractUsage_Gemini_NonStream_MultimodalBuiltin(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usageMetadata":{
	    "promptTokenCount": 81,
	    "candidatesTokenCount": 40,
	    "thoughtsTokenCount": 553,
	    "totalTokenCount": 674,
	    "promptTokensDetails": [
	      {"modality": "TEXT", "tokenCount": 5},
	      {"modality": "IMAGE", "tokenCount": 12},
	      {"modality": "VIDEO", "tokenCount": 34},
	      {"modality": "AUDIO", "tokenCount": 76}
	    ]
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 5 {
		t.Fatalf("input_tokens=%d want=5", u.InputTokens)
	}
	if u.OutputTokens != 593 {
		t.Fatalf("output_tokens=%d want=593", u.OutputTokens)
	}
	if u.TotalTokens != 674 {
		t.Fatalf("total_tokens=%d want=674", u.TotalTokens)
	}
	if got, want := u.FlatFields["image_input_tokens"], 12; got != want {
		t.Fatalf("image_input_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["video_input_tokens"], 34; got != want {
		t.Fatalf("video_input_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["audio_input_tokens"], 76; got != want {
		t.Fatalf("audio_input_tokens=%v want=%v", got, want)
	}
}

func TestExtractUsage_Gemini_NonStream_SnakeCaseUsageIgnored(t *testing.T) {
	meta := &dslmeta.Meta{API: "gemini.generateContent", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "gemini.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage_metadata":{
	    "prompt_token_count": 11,
	    "candidates_token_count": 9,
	    "thoughts_token_count": 3,
	    "total_token_count": 23
	  }
	}`)

	u, _, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if u == nil {
		t.Fatalf("usage nil")
	}
	if u.InputTokens != 0 || u.OutputTokens != 0 || u.TotalTokens != 0 {
		t.Fatalf("unexpected usage from snake_case gemini payload: %+v", *u)
	}
}

func TestUsageDimensionRegistry_AllowsKnownPairs(t *testing.T) {
	reg := NewUsageDimensionRegistry(
		UsageDimension{Dimension: "input", Unit: "token"},
		UsageDimension{Dimension: "image.input", Unit: "token"},
		UsageDimension{Dimension: "video.input", Unit: "token"},
		UsageDimension{Dimension: "audio.input", Unit: "token"},
		UsageDimension{Dimension: "server_tool.web_search", Unit: "call"},
		UsageDimension{Dimension: "image.generate", Unit: "image"},
		UsageDimension{Dimension: "audio.tts", Unit: "second"},
	)

	if !reg.Allows("input", "token") {
		t.Fatalf("expected input token allowed")
	}
	if !reg.Allows("IMAGE.INPUT", "TOKEN") {
		t.Fatalf("expected image.input token allowed")
	}
	if !reg.Allows("video.input", "token") {
		t.Fatalf("expected video.input token allowed")
	}
	if !reg.Allows("audio.input", "token") {
		t.Fatalf("expected audio.input token allowed")
	}
	if !reg.Allows("SERVER_TOOL.WEB_SEARCH", "CALL") {
		t.Fatalf("expected server_tool.web_search call allowed")
	}
	if !reg.Allows("image.generate", "image") {
		t.Fatalf("expected image.generate image allowed")
	}
	if !reg.Allows("audio.tts", "second") {
		t.Fatalf("expected audio.tts second allowed")
	}
}
