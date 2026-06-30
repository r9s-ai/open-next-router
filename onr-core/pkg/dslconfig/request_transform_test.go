package dslconfig

import (
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestJSONOps_SetDelRename(t *testing.T) {
	m := &dslmeta.Meta{
		API:             "chat.completions",
		IsStream:        false,
		OriginModelName: "gpt-4o-mini",
		DSLModelMapped:  "gpt-4o-mini",
	}

	in := map[string]any{
		"stream": false,
		"foo":    "bar",
		"nested": map[string]any{"a": 1},
	}
	out, err := ApplyJSONOps(m, in, []JSONOp{
		{Op: "json_set", Path: "$.stream", ValueExpr: "true"},
		{Op: "json_rename", FromPath: "$.foo", ToPath: "$.baz"},
		{Op: "json_del", Path: "$.nested.a"},
	})
	if err != nil {
		t.Fatalf("ApplyJSONOps: %v", err)
	}
	if out["stream"] != true {
		t.Fatalf("expected stream=true, got %#v", out["stream"])
	}
	if _, ok := out["foo"]; ok {
		t.Fatalf("expected foo removed")
	}
	if out["baz"] != "bar" {
		t.Fatalf("expected baz=bar, got %#v", out["baz"])
	}
	nested, _ := out["nested"].(map[string]any)
	if _, ok := nested["a"]; ok {
		t.Fatalf("expected nested.a removed")
	}
}

func TestModelMap_AppliesToSetPathExpr(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "azure-openai" {
  defaults {
    request {
      model_map "gpt-4o-mini" "gpt4o-mini-prod";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path concat("/openai/deployments/", $request.model_mapped, "/chat/completions");
    }
  }
}
`
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig("azure-openai.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	_ = headers
	_ = response
	_ = perr
	_ = usage
	_ = finish
	_ = balance
	_ = models

	m := &dslmeta.Meta{
		API:             "chat.completions",
		IsStream:        false,
		OriginModelName: "gpt-4o-mini",
		RequestURLPath:  "/v1/chat/completions",
		BaseURL:         "https://example.azure.com",
	}

	if tcfg, ok := req.Select(m); !ok {
		t.Fatalf("expected request transform selected")
	} else {
		tcfg.Apply(m)
	}
	if m.DSLModelMapped != "gpt4o-mini-prod" {
		t.Fatalf("unexpected DSLModelMapped: %q", m.DSLModelMapped)
	}

	if err := routing.Apply(m); err != nil {
		t.Fatalf("routing.Apply: %v", err)
	}
	if m.RequestURLPath != "/openai/deployments/gpt4o-mini-prod/chat/completions" {
		t.Fatalf("unexpected RequestURLPath: %q", m.RequestURLPath)
	}
}

func TestProviderRequestTransformSelectKeepsDSLMatchOrder(t *testing.T) {
	streamTrue := true
	req := &ProviderRequestTransform{
		Matches: []MatchRequestTransform{
			{
				API: "chat.completions",
				Transform: RequestTransform{
					JSONOps: []JSONOp{
						{Op: "json_set", Path: "$.selected", ValueExpr: `"generic"`},
					},
				},
			},
			{
				API:    "chat.completions",
				Stream: &streamTrue,
				Transform: RequestTransform{
					JSONOps: []JSONOp{
						{Op: "json_set", Path: "$.selected", ValueExpr: `"stream"`},
					},
				},
			},
		},
	}

	transform, ok := req.Select(&dslmeta.Meta{API: "chat.completions", IsStream: true})
	if !ok {
		t.Fatalf("expected request transform")
	}
	if len(transform.JSONOps) != 1 || transform.JSONOps[0].ValueExpr != `"generic"` {
		t.Fatalf("expected first DSL match to win, got %#v", transform.JSONOps)
	}
}

func TestSetPathTemplate_AppliesVariables(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
    request {
      model_map "gpt-4o-mini" "gpt4o-mini-prod";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("/openai/deployments/${request.model_mapped}/chat/completions");
    }
  }
}
`
	pf, hasProvider, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validateAndBuildProviderFile: %v", err)
	}
	if !hasProvider {
		t.Fatalf("expected provider")
	}

	m := &dslmeta.Meta{
		API:             "chat.completions",
		IsStream:        false,
		OriginModelName: "gpt-4o-mini",
		RequestURLPath:  "/v1/chat/completions?api-version=2024-10-01",
		BaseURL:         "https://example.azure.com",
	}
	if tcfg, ok := pf.Request.Select(m); !ok {
		t.Fatalf("expected request transform selected")
	} else {
		tcfg.Apply(m)
	}
	if err := pf.Routing.Apply(m); err != nil {
		t.Fatalf("routing.Apply: %v", err)
	}
	if m.RequestURLPath != "/openai/deployments/gpt4o-mini-prod/chat/completions?api-version=2024-10-01" {
		t.Fatalf("unexpected RequestURLPath: %q", m.RequestURLPath)
	}
}

func TestSetPathTemplate_AppliesCredentialAndChannelVariables(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "vertex" {
  defaults {
    upstream_config {
      base_url = "https://aiplatform.googleapis.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("/v1/projects/${credential.project_id}/locations/${channel.location}/publishers/google/models/${request.model_mapped}:generateContent");
    }
  }
}
`
	pf, hasProvider, err := validateAndBuildProviderFile("vertex.conf", conf, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validateAndBuildProviderFile: %v", err)
	}
	if !hasProvider {
		t.Fatalf("expected provider")
	}

	m := &dslmeta.Meta{
		API:                 "chat.completions",
		OriginModelName:     "gemini-2.5-flash",
		DSLModelMapped:      "gemini-2.5-flash-001",
		CredentialProjectID: "proj-1",
		ChannelLocation:     "us-central1",
		RequestURLPath:      "/v1/chat/completions",
	}
	if err := pf.Routing.Apply(m); err != nil {
		t.Fatalf("routing.Apply: %v", err)
	}
	if m.RequestURLPath != "/v1/projects/proj-1/locations/us-central1/publishers/google/models/gemini-2.5-flash-001:generateContent" {
		t.Fatalf("unexpected RequestURLPath: %q", m.RequestURLPath)
	}
}

func TestSetPathTemplate_RejectsEmptyCredentialVariableAtRuntime(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "vertex" {
  defaults {
    upstream_config {
      base_url = "https://aiplatform.googleapis.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("/v1/projects/${credential.project_id}/locations/${channel.location}/publishers/google/models/${request.model_mapped}:generateContent");
    }
  }
}
`
	pf, hasProvider, err := validateAndBuildProviderFile("vertex.conf", conf, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validateAndBuildProviderFile: %v", err)
	}
	if !hasProvider {
		t.Fatalf("expected provider")
	}

	m := &dslmeta.Meta{
		API:             "chat.completions",
		OriginModelName: "gemini-2.5-flash",
		DSLModelMapped:  "gemini-2.5-flash",
		ChannelLocation: "global",
		RequestURLPath:  "/v1/chat/completions",
	}
	err = pf.Routing.Apply(m)
	if err == nil {
		t.Fatalf("expected empty credential.project_id error")
	}
	if !strings.Contains(err.Error(), `template variable "credential.project_id" is empty`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPathConcat_RejectsEmptyModelVariableAtRuntime(t *testing.T) {
	routing := ProviderRouting{
		Matches: []RoutingMatch{
			{
				API:     "chat.completions",
				SetPath: `concat("/v1/models/", $request.model_mapped, ":generateContent")`,
			},
		},
	}
	m := &dslmeta.Meta{
		API:            "chat.completions",
		RequestURLPath: "/v1/chat/completions",
	}
	err := routing.Apply(m)
	if err == nil {
		t.Fatalf("expected empty request.model_mapped error")
	}
	if !strings.Contains(err.Error(), `template variable "request.model_mapped" is empty`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPathConcat_AppliesTaskUpstreamID(t *testing.T) {
	routing := ProviderRouting{
		Matches: []RoutingMatch{
			{
				API:     "gemini.getOperation",
				SetPath: `concat("/v1beta/", $task.upstream_id)`,
			},
		},
	}
	m := &dslmeta.Meta{
		API:            "gemini.getOperation",
		RequestURLPath: "/v1beta/query",
		Task: dslmeta.TaskMeta{
			UpstreamID: "models/veo/operations/op-1",
		},
	}
	if err := routing.Apply(m); err != nil {
		t.Fatalf("Apply returned err: %v", err)
	}
	if m.RequestURLPath != "/v1beta/models/veo/operations/op-1" {
		t.Fatalf("RequestURLPath=%q", m.RequestURLPath)
	}
}

func TestSetPathTemplate_RejectsUnknownVariable(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("/v1/${request.unknown}/chat/completions");
    }
  }
}
`
	_, _, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "unsupported template variable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPathTemplate_EscapesLiteralPlaceholder(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("/literal/\${request.model_mapped}");
    }
  }
}
`
	pf, hasProvider, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("validateAndBuildProviderFile: %v", err)
	}
	if !hasProvider {
		t.Fatalf("expected provider")
	}
	m := &dslmeta.Meta{
		API:             "chat.completions",
		IsStream:        false,
		OriginModelName: "gpt-4o-mini",
		RequestURLPath:  "/v1/chat/completions",
	}
	if err := pf.Routing.Apply(m); err != nil {
		t.Fatalf("routing.Apply: %v", err)
	}
	if m.RequestURLPath != "/literal/$%7Brequest.model_mapped%7D" {
		t.Fatalf("unexpected RequestURLPath: %q", m.RequestURLPath)
	}
}

func TestSetPathTemplate_RejectsNonPathTemplate(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path template("v1/${request.model_mapped}/chat/completions");
    }
  }
}
`
	_, _, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "path must start with '/'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPath_RejectsBareVariable(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path $request.model_mapped;
    }
  }
}
`
	_, _, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "bare variables are not valid set_path expressions") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPath_RejectsUnquotedPathWithVariable(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "templated" {
  defaults {
    upstream_config {
      base_url = "https://api.example.com";
    }
  }

  match api = "chat.completions" {
    upstream {
      set_path /v1/$request.model_mapped/chat/completions;
    }
  }
}
`
	_, _, err := validateAndBuildProviderFile("templated.conf", conf, nil, nil, nil, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "unquoted path literals cannot contain '$'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestJSONSetIfAbsent_Parsed(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "t" {
  defaults {
    request {
      json_set_if_absent "$.instructions" "";
    }
  }
}
`
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig("t.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	_ = routing
	_ = headers
	_ = response
	_ = perr
	_ = usage
	_ = finish
	_ = balance
	_ = models
	if len(req.Defaults.JSONOps) != 1 {
		t.Fatalf("expected 1 json op, got %d", len(req.Defaults.JSONOps))
	}
	op := req.Defaults.JSONOps[0]
	if op.Op != "json_set_if_absent" {
		t.Fatalf("unexpected op: %q", op.Op)
	}
	if op.Path != "$.instructions" {
		t.Fatalf("unexpected path: %q", op.Path)
	}
	if op.ValueExpr != "\"\"" {
		t.Fatalf("unexpected value expr: %q", op.ValueExpr)
	}
}

func TestRequestJSONWrapInputText_Parsed(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "t" {
  defaults {
    request {
      json_wrap_input_text "$.input";
    }
  }
}
`
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig("t.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	_ = routing
	_ = headers
	_ = response
	_ = perr
	_ = usage
	_ = finish
	_ = balance
	_ = models
	if len(req.Defaults.JSONOps) != 1 {
		t.Fatalf("expected 1 json op, got %d", len(req.Defaults.JSONOps))
	}
	op := req.Defaults.JSONOps[0]
	if op.Op != "json_wrap_input_text" {
		t.Fatalf("unexpected op: %q", op.Op)
	}
	if op.Path != "$.input" {
		t.Fatalf("unexpected path: %q", op.Path)
	}
}

func TestRequestJSONSetHeaderValues_Parsed(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "t" {
  defaults {
    request {
      json_set_header_values "$.anthropic_beta" "anthropic-beta";
      json_filter_values "$.anthropic_beta" "computer-use-2025-01-24" "context-management-2025-06-27";
    }
  }
}
`
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig("t.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	_ = routing
	_ = headers
	_ = response
	_ = perr
	_ = usage
	_ = finish
	_ = balance
	_ = models
	if len(req.Defaults.JSONOps) != 2 {
		t.Fatalf("expected 2 json ops, got %d", len(req.Defaults.JSONOps))
	}
	op := req.Defaults.JSONOps[0]
	if op.Op != "json_set_header_values" {
		t.Fatalf("unexpected op: %q", op.Op)
	}
	if op.Path != "$.anthropic_beta" || op.HeaderName != "anthropic-beta" {
		t.Fatalf("unexpected op target: %#v", op)
	}
	filterOp := req.Defaults.JSONOps[1]
	if filterOp.Op != "json_filter_values" || filterOp.Path != "$.anthropic_beta" {
		t.Fatalf("unexpected filter op: %#v", filterOp)
	}
	if len(filterOp.Patterns) != 2 || filterOp.Patterns[0] != "computer-use-2025-01-24" || filterOp.Patterns[1] != "context-management-2025-06-27" {
		t.Fatalf("unexpected patterns: %#v", filterOp.Patterns)
	}
}

func TestRequestJSONSetHeaderValues_RejectsUnusedExtraArgs(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "t" {
  defaults {
    request {
      json_set_header_values "$.anthropic_beta" "anthropic-beta" "computer-use-2025-01-24";
    }
  }
}
`
	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("t.conf", conf)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "use json_filter_values to filter values") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequestAfterReqMap_Parsed(t *testing.T) {
	conf := `
syntax "next-router/0.1";

provider "t" {
  defaults {
    request {
      json_del "$.stream_options";
      after_req_map {
        json_set "$.anthropic_version" "bedrock-2023-05-31";
        json_set_header_values "$.anthropic_beta" "anthropic-beta";
        json_filter_values "$.anthropic_beta" "computer-use-2025-01-24";
        json_del_with_condition "$.tools" "type" "web_search*" "web_fetch*";
        json_del_if_missing "$.tool_choice" "$.tools";
      }
    }
  }
}
`
	routing, headers, req, response, perr, usage, finish, balance, models, err := parseProviderConfig("t.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}
	_ = routing
	_ = headers
	_ = response
	_ = perr
	_ = usage
	_ = finish
	_ = balance
	_ = models
	if len(req.Defaults.JSONOps) != 1 {
		t.Fatalf("expected 1 json op, got %d", len(req.Defaults.JSONOps))
	}
	if got := req.Defaults.JSONOps[0].Op; got != "json_del" {
		t.Fatalf("JSONOps[0].Op=%q", got)
	}
	if len(req.Defaults.AfterReqMapJSONOps) != 5 {
		t.Fatalf("expected 5 after_req_map json ops, got %d", len(req.Defaults.AfterReqMapJSONOps))
	}
	if got := req.Defaults.AfterReqMapJSONOps[0].Op; got != "json_set" {
		t.Fatalf("AfterReqMapJSONOps[0].Op=%q", got)
	}
	if got := req.Defaults.AfterReqMapJSONOps[3].Op; got != "json_del_with_condition" {
		t.Fatalf("AfterReqMapJSONOps[3].Op=%q", got)
	}
	if got := req.Defaults.AfterReqMapJSONOps[4].Op; got != "json_del_if_missing" {
		t.Fatalf("AfterReqMapJSONOps[4].Op=%q", got)
	}
}

func TestValidateProviderFile_RequestJSONWrapInputTextRejectsInvalidPath(t *testing.T) {
	path := writeProviderFile(t, "t.conf", `
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    request {
      json_wrap_input_text "$.input[0]";
    }
  }
}
`)
	_, err := ValidateProviderFile(path)
	if err == nil || !strings.Contains(err.Error(), "array indexes") {
		t.Fatalf("ValidateProviderFile err=%v, want array index validation error", err)
	}
}
