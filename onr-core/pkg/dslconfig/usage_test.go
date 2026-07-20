package dslconfig

import (
	"os"
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
	      "cached_tokens": 5,
	      "cache_write_tokens": 3
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
	if u.InputTokenDetails.CacheWriteTokens != 3 {
		t.Fatalf("cache_write_tokens=%d want=3", u.InputTokenDetails.CacheWriteTokens)
	}
}

func TestExtractUsage_OpenAIChatCompletionsCacheWriteMainPathWins(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "prompt_tokens": 11,
	    "completion_tokens": 5,
	    "prompt_tokens_details": {
	      "cached_tokens": 2,
	      "cache_write_tokens": 3
	    },
	    "input_tokens_details": {
	      "cached_tokens": 97,
	      "cache_write_tokens": 99
	    }
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if u == nil || u.InputTokenDetails == nil {
		t.Fatalf("expected usage details, got=%+v", u)
	}
	if u.InputTokens != 11 || u.OutputTokens != 5 || u.TotalTokens != 16 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 2 || u.InputTokenDetails.CachedTokens != 2 {
		t.Fatalf("cached=%d details=%+v want=2", cached, u.InputTokenDetails)
	}
	if u.InputTokenDetails.CacheWriteTokens != 3 {
		t.Fatalf("cache_write_tokens=%d want=3", u.InputTokenDetails.CacheWriteTokens)
	}
}

func TestExtractUsage_OpenAIResponsesCacheWrite(t *testing.T) {
	meta := &dslmeta.Meta{API: "responses", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "usage": {
	    "input_tokens": 13,
	    "output_tokens": 7,
	    "input_tokens_details": {
	      "cached_tokens": 4,
	      "cache_write_tokens": 6
	    }
	  }
	}`)

	u, cached, err := ExtractUsage(meta, cfg, resp)
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if u == nil || u.InputTokenDetails == nil {
		t.Fatalf("expected usage details, got=%+v", u)
	}
	if u.InputTokens != 13 || u.OutputTokens != 7 || u.TotalTokens != 20 {
		t.Fatalf("unexpected usage: %+v", *u)
	}
	if cached != 4 || u.InputTokenDetails.CachedTokens != 4 {
		t.Fatalf("cached=%d details=%+v want=4", cached, u.InputTokenDetails)
	}
	if u.InputTokenDetails.CacheWriteTokens != 6 {
		t.Fatalf("cache_write_tokens=%d want=6", u.InputTokenDetails.CacheWriteTokens)
	}
}

func TestExtractUsage_OpenAIResponsesOptionalCacheFields(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantCached int
		wantWrite  int
	}{
		{
			name:       "cache read only",
			body:       `{"usage":{"input_tokens":13,"output_tokens":7,"input_tokens_details":{"cached_tokens":4}}}`,
			wantCached: 4,
		},
		{
			name:      "cache write only",
			body:      `{"usage":{"input_tokens":13,"output_tokens":7,"input_tokens_details":{"cache_write_tokens":6}}}`,
			wantWrite: 6,
		},
		{
			name: "no cache fields",
			body: `{"usage":{"input_tokens":13,"output_tokens":7}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := &dslmeta.Meta{API: "responses", IsStream: false}
			cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

			u, cached, err := ExtractUsage(meta, cfg, []byte(tc.body))
			if err != nil {
				t.Fatalf("ExtractUsage: %v", err)
			}
			if u == nil {
				t.Fatal("expected usage")
			}
			if cached != tc.wantCached {
				t.Fatalf("cached=%d want=%d", cached, tc.wantCached)
			}
			if tc.wantCached == 0 && tc.wantWrite == 0 {
				if u.InputTokenDetails != nil &&
					(u.InputTokenDetails.CachedTokens != 0 || u.InputTokenDetails.CacheWriteTokens != 0) {
					t.Fatalf("unexpected input token details: %+v", u.InputTokenDetails)
				}
				return
			}
			if u.InputTokenDetails == nil {
				t.Fatal("expected input token details")
			}
			if u.InputTokenDetails.CachedTokens != tc.wantCached {
				t.Fatalf("cached token details=%d want=%d", u.InputTokenDetails.CachedTokens, tc.wantCached)
			}
			if u.InputTokenDetails.CacheWriteTokens != tc.wantWrite {
				t.Fatalf("cache_write_tokens=%d want=%d", u.InputTokenDetails.CacheWriteTokens, tc.wantWrite)
			}
		})
	}
}

func TestExtractUsageObject_OpenAI_NonStream(t *testing.T) {
	meta := &dslmeta.Meta{API: "chat.completions", IsStream: false}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	root := map[string]any{
		"usage": map[string]any{
			"input_tokens":  8,
			"output_tokens": 9,
			"input_tokens_details": map[string]any{
				"cached_tokens":      5,
				"cache_write_tokens": 3,
			},
		},
	}

	u, cached, err := ExtractUsageObject(meta, cfg, root)
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
	if u.InputTokenDetails.CacheWriteTokens != 3 {
		t.Fatalf("cache_write_tokens=%d want=3", u.InputTokenDetails.CacheWriteTokens)
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
	if got, want := u.FlatFields["output_image_tokens"], 4096; got != want {
		t.Fatalf("output_image_tokens=%v want=%v", got, want)
	}
	found := false
	foundOutputImage := false
	for _, fact := range u.DebugFacts {
		if fact.Dimension == "image.generate" && fact.Unit == "image" && fact.Quantity == 1 {
			found = true
		}
		if fact.Dimension == "output.image" && fact.Unit == "token" && fact.Quantity == 4096 {
			foundOutputImage = true
		}
	}
	if !found {
		t.Fatalf("expected image.generate image fact, got=%#v", u.DebugFacts)
	}
	if !foundOutputImage {
		t.Fatalf("expected output.image token fact, got=%#v", u.DebugFacts)
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
	meta := &dslmeta.Meta{
		API:          "audio.translations",
		IsStream:     false,
		DerivedUsage: map[string]any{"request_audio_duration_seconds": 4},
	}
	cfg, _ := mustLoadProviderMatchConfigs(t, "openai.conf", meta.API, meta.IsStream)

	resp := []byte(`{
	  "text":"hello translated"
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
	u, _, err := extractUsageFromRootsWithEvent(meta, "response.completed", *cfg, nil, root, nil)
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
	if u.InputTokens != 81 {
		t.Fatalf("input_tokens=%d want=81", u.InputTokens)
	}
	if u.OutputTokens != 593 {
		t.Fatalf("output_tokens=%d want=593", u.OutputTokens)
	}
	if u.TotalTokens != 674 {
		t.Fatalf("total_tokens=%d want=674", u.TotalTokens)
	}
	if got, want := u.FlatFields["input_image_tokens"], 12; got != want {
		t.Fatalf("input_image_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["input_video_tokens"], 34; got != want {
		t.Fatalf("input_video_tokens=%v want=%v", got, want)
	}
	if got, want := u.FlatFields["input_audio_tokens"], 76; got != want {
		t.Fatalf("input_audio_tokens=%v want=%v", got, want)
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
		UsageDimension{Dimension: "input.image", Unit: "token"},
		UsageDimension{Dimension: "input.video", Unit: "token"},
		UsageDimension{Dimension: "input.audio", Unit: "token"},
		UsageDimension{Dimension: "output.image", Unit: "token"},
		UsageDimension{Dimension: "output.audio", Unit: "token"},
		UsageDimension{Dimension: "output.video", Unit: "token"},
		UsageDimension{Dimension: "server_tool.web_search", Unit: "call"},
		UsageDimension{Dimension: "image.generate", Unit: "image"},
		UsageDimension{Dimension: "audio.tts", Unit: "second"},
	)

	if !reg.Allows("input", "token") {
		t.Fatalf("expected input token allowed")
	}
	if !reg.Allows("INPUT.IMAGE", "TOKEN") {
		t.Fatalf("expected input.image token allowed")
	}
	if !reg.Allows("input.video", "token") {
		t.Fatalf("expected input.video token allowed")
	}
	if !reg.Allows("input.audio", "token") {
		t.Fatalf("expected input.audio token allowed")
	}
	if reg.Allows("image.input", "token") {
		t.Fatalf("did not expect image.input token allowed")
	}
	if !reg.Allows("OUTPUT.IMAGE", "TOKEN") {
		t.Fatalf("expected output.image token allowed")
	}
	if !reg.Allows("output.audio", "token") {
		t.Fatalf("expected output.audio token allowed")
	}
	if !reg.Allows("output.video", "token") {
		t.Fatalf("expected output.video token allowed")
	}
	if reg.Allows("image.output", "token") {
		t.Fatalf("expected image.output token not allowed")
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

// TestUsageExtractConfig_SourceModeSetAfterNamedModeResolution verifies that
// UsageExtractConfig.SourceMode is populated with the referenced usage_mode
// name after resolution, enabling callers to identify which named preset was used.
func TestUsageExtractConfig_SourceModeSetAfterNamedModeResolution(t *testing.T) {
	t.Parallel()
	cases := []struct {
		api            string
		stream         bool
		wantSourceMode string
	}{
		{api: "claude.messages", stream: false, wantSourceMode: "anthropic_messages"},
		{api: "claude.messages", stream: true, wantSourceMode: "anthropic_messages_stream"},
		{api: "chat.completions", stream: false, wantSourceMode: "openai_chat_completions"},
	}
	for _, tc := range cases {
		t.Run(tc.api+"/stream="+boolStr(tc.stream), func(t *testing.T) {
			t.Parallel()
			cfg, _ := mustLoadProviderMatchConfigs(t, "anthropic.conf", tc.api, tc.stream)
			if cfg.SourceMode != tc.wantSourceMode {
				t.Fatalf("SourceMode=%q want=%q", cfg.SourceMode, tc.wantSourceMode)
			}
		})
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestMergeUsageConfig_SourceModeOverride verifies that mergeUsageConfig
// applies a non-empty override.SourceMode over the base value.
func TestMergeUsageConfig_SourceModeOverride(t *testing.T) {
	t.Parallel()
	base := UsageExtractConfig{SourceMode: "base_mode", InputTokensPath: "$.base"}
	override := UsageExtractConfig{SourceMode: "override_mode"}
	got := mergeUsageConfig(base, override)
	if got.SourceMode != "override_mode" {
		t.Fatalf("SourceMode=%q want=%q", got.SourceMode, "override_mode")
	}
	if got.InputTokensPath != "$.base" {
		t.Fatalf("InputTokensPath=%q want=%q (base should be preserved)", got.InputTokensPath, "$.base")
	}
}

// TestMergeUsageConfig_EmptySourceModeKeepsBase verifies that an empty
// override.SourceMode does not clear the base SourceMode.
func TestMergeUsageConfig_EmptySourceModeKeepsBase(t *testing.T) {
	t.Parallel()
	base := UsageExtractConfig{SourceMode: "base_mode"}
	override := UsageExtractConfig{SourceMode: ""}
	got := mergeUsageConfig(base, override)
	if got.SourceMode != "base_mode" {
		t.Fatalf("SourceMode=%q want=%q (empty override should not clear base)", got.SourceMode, "base_mode")
	}
}

// TestResolveUsageModeRegistry_SourceModePropagatedToProvider verifies end-to-end
// that a provider using a global usage_mode preset has SourceMode set on the
// resolved config after registry reload.
func TestResolveUsageModeRegistry_SourceModePropagatedToProvider(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "testprovider.conf")
	content := `syntax "next-router/0.1";

usage_mode "my_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.prompt_tokens";
  usage_fact output token path="$.usage.completion_tokens";
}

provider "testprovider" {
  defaults {
    upstream_config { base_url = "https://example.com"; }
    metrics { usage_extract my_tokens; }
  }
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	reg := NewRegistry()
	if _, err := reg.ReloadFromFile(path); err != nil {
		t.Fatalf("ReloadFromFile: %v", err)
	}
	pf, ok := reg.GetProvider("testprovider")
	if !ok {
		t.Fatalf("provider not found")
	}
	cfg, ok := pf.Usage.Select(&dslmeta.Meta{API: "chat.completions"})
	if !ok {
		t.Fatalf("no usage config for defaults")
	}
	if cfg.SourceMode != "my_tokens" {
		t.Fatalf("SourceMode=%q want=%q", cfg.SourceMode, "my_tokens")
	}
}
