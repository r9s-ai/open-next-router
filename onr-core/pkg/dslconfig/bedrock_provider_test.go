package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmetadata"
)

func TestValidateProviderFile_BedrockRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "aws-bedrock.conf")
	if err := os.WriteFile(path, []byte(`syntax "next-router/0.1";

provider "aws-bedrock" {
  defaults {
    upstream_config {
      transport aws_sdk;
    }
    auth {
      auth_sigv4_bedrock;
    }
    response {
      resp_passthrough;
    }
  }

  match api = "claude.messages" stream = false {
    request {
      model_map_default $request.model;
      json_del "$.model";
    }
    upstream {
      set_path template("/model/${request.model_mapped}/invoke");
    }
  }

  match api = "chat.completions" stream = true {
    request {
      model_map "claude-3-5-sonnet-20241022" "anthropic.claude-3-5-sonnet-20241022-v2:0";
      json_set "$.stream" true;
    }
    upstream {
      set_path "/openai/v1/chat/completions";
    }
  }

  match api = "responses" {
    upstream {
      set_path "/custom/v1/anything";
    }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if !pf.Headers.Defaults.AWSSigV4 {
		t.Fatalf("expected AWS SigV4 auth")
	}
	if len(pf.Routing.Matches) != 3 {
		t.Fatalf("matches=%#v", pf.Routing.Matches)
	}

	meta := &dslmeta.Meta{
		API:             "chat.completions",
		IsStream:        true,
		OriginModelName: "claude-3-5-sonnet-20241022",
		DSLModelMapped:  "anthropic.claude-3-5-sonnet-20241022-v2:0",
		RequestURLPath:  "/v1/chat/completions",
	}
	if err := pf.Routing.Apply(meta); err != nil {
		t.Fatalf("Routing.Apply: %v", err)
	}
	if meta.UpstreamTransport != "aws_sdk" {
		t.Fatalf("UpstreamTransport=%q", meta.UpstreamTransport)
	}
	if meta.RequestURLPath != "/openai/v1/chat/completions" {
		t.Fatalf("RequestURLPath=%q", meta.RequestURLPath)
	}

	exported := ExportProviderMetadata(pf)
	if exported.Auth == nil || exported.Auth.Type != "aws_sigv4" || exported.Auth.Service != "bedrock" || !exported.Auth.RequiresRegion {
		t.Fatalf("auth metadata=%#v", exported.Auth)
	}
	if exported.Upstream == nil || exported.Upstream.Transport != "aws_sdk" {
		t.Fatalf("upstream metadata=%#v", exported.Upstream)
	}
	transform, ok := dslmetadata.SelectRequestTransform(exported.Request, "claude.messages", false)
	if !ok {
		t.Fatalf("expected claude.messages request metadata: %#v", exported.Request)
	}
	if transform.ModelMap.DefaultExpr != "$request.model" {
		t.Fatalf("default expr=%q", transform.ModelMap.DefaultExpr)
	}
	if len(transform.JSONOps) != 1 || transform.JSONOps[0].Op != "json_del" || transform.JSONOps[0].Path != "$.model" {
		t.Fatalf("unexpected json ops: %#v", transform.JSONOps)
	}
}

func TestValidateProviderFile_BedrockRejectsInvalidCombinations(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "stream path needs stream match",
			content: `provider "aws-bedrock" {
  defaults { upstream_config { transport aws_sdk; } auth { auth_sigv4_bedrock; } }
  match api = "chat.completions" stream = false {
    upstream { set_path template("/model/${request.model_mapped}/invoke-with-response-stream"); }
  }
}`,
			want: "requires match stream = true",
		},
		{
			name: "auth mix",
			content: `provider "aws-bedrock" {
  defaults { upstream_config { transport aws_sdk; } auth { auth_sigv4_bedrock; auth_bearer; } }
  match api = "chat.completions" stream = false {
    upstream { set_path template("/model/${request.model_mapped}/invoke"); }
  }
}`,
			want: "cannot be combined",
		},
		{
			name: "query routing",
			content: `provider "aws-bedrock" {
  defaults { upstream_config { transport aws_sdk; } auth { auth_sigv4_bedrock; } }
  match api = "chat.completions" stream = false {
    upstream { set_path template("/model/${request.model_mapped}/invoke"); set_query x "1"; }
  }
}`,
			want: "does not support query routing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "aws-bedrock.conf")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatalf("write provider file: %v", err)
			}
			_, err := ValidateProviderFile(path)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want contains %q", err, tt.want)
			}
		})
	}
}
