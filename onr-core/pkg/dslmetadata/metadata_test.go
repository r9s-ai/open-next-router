package dslmetadata

import "testing"

func TestSelectRequestTransform(t *testing.T) {
	streamTrue := true
	cfg := &ProviderRequest{
		Defaults: RequestTransform{
			ModelMap: ModelMap{
				Map:         map[string]string{"base": "mapped-base"},
				DefaultExpr: `concat("base-", $request.model)`,
			},
			JSONOps:    []JSONOp{{Op: "json_set", Path: "$.source", ValueExpr: `"default"`}},
			ReqMapMode: "openai_chat_to_responses",
		},
		Matches: []RequestTransformMatch{
			{
				API: "chat.completions",
				Transform: RequestTransform{
					ModelMap: ModelMap{
						Map:         map[string]string{"base": "mapped-override", "match": "mapped-match"},
						DefaultExpr: `concat("match-", $request.model)`,
					},
					JSONOps:            []JSONOp{{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model_mapped"}},
					AfterReqMapJSONOps: []JSONOp{{Op: "json_set", Path: "$.after", ValueExpr: "true"}},
				},
			},
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Transform: RequestTransform{
					JSONOps: []JSONOp{{Op: "json_set", Path: "$.selected", ValueExpr: `"stream"`}},
				},
			},
		},
	}

	transform, ok := SelectRequestTransform(cfg, "chat.completions", true)
	if !ok {
		t.Fatalf("expected transform")
	}
	if got := transform.ModelMap.Map["base"]; got != "mapped-override" {
		t.Fatalf("base model map=%q", got)
	}
	if got := transform.ModelMap.Map["match"]; got != "mapped-match" {
		t.Fatalf("match model map=%q", got)
	}
	if transform.ModelMap.DefaultExpr != `concat("match-", $request.model)` {
		t.Fatalf("default expr=%q", transform.ModelMap.DefaultExpr)
	}
	if len(transform.JSONOps) != 2 {
		t.Fatalf("json ops=%#v", transform.JSONOps)
	}
	if transform.JSONOps[1].ValueExpr != "$request.model_mapped" {
		t.Fatalf("expected generic match to win by DSL order, got %#v", transform.JSONOps)
	}
	if len(transform.AfterReqMapJSONOps) != 1 {
		t.Fatalf("after req_map ops=%#v", transform.AfterReqMapJSONOps)
	}
	if transform.ReqMapMode != "openai_chat_to_responses" {
		t.Fatalf("req_map mode=%q", transform.ReqMapMode)
	}
}

func TestSelectRequestTransformDefaultsAndEmpty(t *testing.T) {
	transform, ok := SelectRequestTransform(&ProviderRequest{
		Defaults: RequestTransform{
			ModelMap: ModelMap{DefaultExpr: `concat("mapped-", $request.model)`},
		},
	}, "responses", false)
	if !ok {
		t.Fatalf("expected defaults transform")
	}
	if transform.ModelMap.DefaultExpr == "" {
		t.Fatalf("expected default expr")
	}

	if _, ok := SelectRequestTransform(&ProviderRequest{}, "responses", false); ok {
		t.Fatalf("empty request config should not match")
	}
}

func TestSelectRequestTransformEmptyAPIMatch(t *testing.T) {
	streamTrue := true
	cfg := NormalizeProviderConfig(ProviderConfig{
		Request: &ProviderRequest{
			Matches: []RequestTransformMatch{
				{
					API:    " ",
					Stream: &streamTrue,
					Transform: RequestTransform{
						JSONOps: []JSONOp{{Op: "json_set", Path: "$.selected", ValueExpr: `"stream-any-api"`}},
					},
				},
				{
					API: "chat.completions",
					Transform: RequestTransform{
						JSONOps: []JSONOp{{Op: "json_set", Path: "$.selected", ValueExpr: `"chat"`}},
					},
				},
			},
		},
	})
	if cfg.Request == nil || len(cfg.Request.Matches) != 2 || cfg.Request.Matches[0].API != "" {
		t.Fatalf("empty api match was not preserved: %#v", cfg.Request)
	}
	transform, ok := SelectRequestTransform(cfg.Request, "chat.completions", true)
	if !ok {
		t.Fatalf("expected transform")
	}
	if len(transform.JSONOps) != 1 || transform.JSONOps[0].ValueExpr != `"stream-any-api"` {
		t.Fatalf("expected empty api match to win by DSL order, got %#v", transform.JSONOps)
	}
}

func TestSelectRoutePreservesDSLOrder(t *testing.T) {
	streamTrue := true
	route, ok := SelectRoute([]ProviderRoute{
		{API: "chat.completions", Path: "/generic/{model}"},
		{API: "chat.completions", Stream: &streamTrue, Path: "/stream/{model}"},
	}, "chat.completions", true)
	if !ok {
		t.Fatalf("expected route")
	}
	if route.Path != "/generic/{model}" {
		t.Fatalf("path=%q", route.Path)
	}
}

func TestSelectUsageFacts(t *testing.T) {
	streamTrue := true
	cfg := &ProviderUsageFacts{
		Defaults: []UsageFact{{Dimension: "input", Unit: "token", Path: "$.usage.input"}},
		Matches: []UsageFactMatch{
			{
				API:   "chat.completions",
				Facts: []UsageFact{{Dimension: "server_tool.web_search", Unit: "call", CountPath: "$.tools[*]"}},
			},
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Facts:  []UsageFact{{Dimension: "output", Unit: "token", Path: "$.usage.output"}},
			},
		},
	}

	facts, ok := SelectUsageFacts(cfg, "chat.completions", true)
	if !ok {
		t.Fatalf("expected facts")
	}
	if len(facts) != 2 {
		t.Fatalf("facts=%#v", facts)
	}
	if facts[1].Dimension != "server_tool.web_search" {
		t.Fatalf("expected generic match to win by DSL order, got %#v", facts)
	}
}

func TestSelectUsageFactsEmptyAPIMatch(t *testing.T) {
	streamFalse := false
	cfg := NormalizeProviderConfig(ProviderConfig{
		UsageFacts: &ProviderUsageFacts{
			Matches: []UsageFactMatch{
				{
					API:    " ",
					Stream: &streamFalse,
					Facts:  []UsageFact{{Dimension: "audio.tts", Unit: "second", Source: "derived", Path: "$.audio_duration_seconds"}},
				},
				{
					API:   "audio.speech",
					Facts: []UsageFact{{Dimension: "output", Unit: "token", Path: "$.usage.output"}},
				},
			},
		},
	})
	if cfg.UsageFacts == nil || len(cfg.UsageFacts.Matches) != 2 || cfg.UsageFacts.Matches[0].API != "" {
		t.Fatalf("empty api usage match was not preserved: %#v", cfg.UsageFacts)
	}
	facts, ok := SelectUsageFacts(cfg.UsageFacts, "audio.speech", false)
	if !ok {
		t.Fatalf("expected facts")
	}
	if len(facts) != 1 || facts[0].Dimension != "audio.tts" {
		t.Fatalf("expected empty api usage match to win by DSL order, got %#v", facts)
	}
}

func TestNormalizeProviderConfig(t *testing.T) {
	streamFalse := false
	cfg := NormalizeProviderConfig(ProviderConfig{
		Metadata: &ProviderMetadata{ProviderFamily: " OpenAI-Compatible ", SignalProfile: " Generic "},
		Auth:     &ProviderAuth{Type: " OAuth ", Header: " Authorization ", Mode: " Google_Service_Account_File ", Scope: " scope ", TokenURL: " https://token.example "},
		Routes: []ProviderRoute{
			{API: " Chat.Completions ", Stream: &streamFalse, Path: " /chat "},
			{API: " ", Path: "/ignored"},
		},
		Request: &ProviderRequest{
			Defaults: RequestTransform{
				ModelMap: ModelMap{
					Map:         map[string]string{" model ": " mapped "},
					DefaultExpr: " default ",
				},
				JSONOps: []JSONOp{{Op: " json_set ", Path: " $.x ", Patterns: []string{" a ", ""}}},
			},
			Matches: []RequestTransformMatch{
				{API: " Chat.Completions ", Transform: RequestTransform{JSONOps: []JSONOp{{Op: " json_replace ", Path: " $.model "}}}},
				{API: " Responses ", Transform: RequestTransform{}},
			},
		},
		Models:     &ProviderModels{Mode: " OpenAI "},
		Balance:    &ProviderBalance{Path: " /dashboard/billing/credit_grants "},
		UsageFacts: &ProviderUsageFacts{Defaults: []UsageFact{{Dimension: " INPUT ", Unit: " TOKEN ", Source: " RESPONSE ", Attributes: map[string]string{" Kind ": " Text "}}}},
	})

	if cfg.Metadata.ProviderFamily != "openai-compatible" || cfg.Metadata.SignalProfile != "generic" {
		t.Fatalf("metadata=%#v", cfg.Metadata)
	}
	if cfg.Auth.Type != "oauth" || cfg.Auth.Mode != "google_service_account_file" || cfg.Auth.Header != "Authorization" {
		t.Fatalf("auth=%#v", cfg.Auth)
	}
	if len(cfg.Routes) != 1 || cfg.Routes[0].API != "chat.completions" || cfg.Routes[0].Path != "/chat" {
		t.Fatalf("routes=%#v", cfg.Routes)
	}
	if got := cfg.Request.Defaults.ModelMap.Map["model"]; got != "mapped" {
		t.Fatalf("model map=%#v", cfg.Request.Defaults.ModelMap.Map)
	}
	if len(cfg.Request.Matches) != 1 || cfg.Request.Matches[0].API != "chat.completions" {
		t.Fatalf("matches=%#v", cfg.Request.Matches)
	}
	if cfg.Models.Path != "/v1/models" || len(cfg.Models.IDPaths) != 1 {
		t.Fatalf("models=%#v", cfg.Models)
	}
	if cfg.Balance.Mode != "custom" || cfg.Balance.Method != "GET" {
		t.Fatalf("balance=%#v", cfg.Balance)
	}
	if cfg.UsageFacts.Defaults[0].Attributes["kind"] != "text" {
		t.Fatalf("usage facts=%#v", cfg.UsageFacts.Defaults)
	}
}

func TestCanonicalJSONCompare(t *testing.T) {
	left := map[string]any{
		"b": "2",
		"a": map[string]any{"y": "2", "x": "1"},
	}
	right := map[string]any{
		"a": map[string]any{"x": "1", "y": "2"},
		"b": "2",
	}
	if !EqualCanonical(left, right) {
		t.Fatalf("expected maps with different key order to be equal")
	}

	if EqualCanonical(
		[]ProviderRoute{{API: "chat.completions", Path: "/a"}, {API: "responses", Path: "/b"}},
		[]ProviderRoute{{API: "responses", Path: "/b"}, {API: "chat.completions", Path: "/a"}},
	) {
		t.Fatalf("array order must remain significant")
	}
}
