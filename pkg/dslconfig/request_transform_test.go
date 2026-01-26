package dslconfig

import (
	"testing"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
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
	outAny, err := ApplyJSONOps(m, in, []JSONOp{
		{Op: "json_set", Path: "$.stream", ValueExpr: "true"},
		{Op: "json_rename", FromPath: "$.foo", ToPath: "$.baz"},
		{Op: "json_del", Path: "$.nested.a"},
	})
	if err != nil {
		t.Fatalf("ApplyJSONOps: %v", err)
	}
	out, ok := outAny.(map[string]any)
	if !ok {
		t.Fatalf("unexpected output type: %T", outAny)
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
	routing, _, req, _, _, _, err := parseProviderConfig("azure-openai.conf", conf)
	if err != nil {
		t.Fatalf("parseProviderConfig: %v", err)
	}

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
