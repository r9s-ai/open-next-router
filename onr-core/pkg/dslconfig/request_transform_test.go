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
		ActualModelName: "gpt-4o-mini",
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
		ActualModelName: "gpt-4o-mini",
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
	if len(req.Defaults.AfterReqMapJSONOps) != 4 {
		t.Fatalf("expected 4 after_req_map json ops, got %d", len(req.Defaults.AfterReqMapJSONOps))
	}
	if got := req.Defaults.AfterReqMapJSONOps[0].Op; got != "json_set" {
		t.Fatalf("AfterReqMapJSONOps[0].Op=%q", got)
	}
	if got := req.Defaults.AfterReqMapJSONOps[3].Op; got != "json_del_with_condition" {
		t.Fatalf("AfterReqMapJSONOps[3].Op=%q", got)
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
