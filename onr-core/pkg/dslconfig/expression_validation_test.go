package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviderFile_TemplateUnknownVariables(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "request_header",
			body: `request { set_header "x-model" template("${request.unknown}"); }`,
		},
		{
			name: "request_json_value",
			body: `request { json_set "$.model" template("${request.unknown}"); }`,
		},
		{
			name: "response_json_value",
			body: `response { json_replace "$.model" template("${request.unknown}"); }`,
		},
		{
			name: "oauth_value",
			body: `auth {
      oauth_mode custom;
      oauth_token_url template("https://example.com/${request.unknown}");
      oauth_form "grant_type" "refresh_token";
    }`,
		},
		{
			name: "models_path",
			body: `models {
      models_mode custom;
      path template("/v1/${request.unknown}/models");
      id_path "$.data[*].id";
    }`,
		},
		{
			name: "balance_path",
			body: `balance {
      balance_mode custom;
      path template("/v1/${request.unknown}/balance");
      balance_path "$.data.total";
    }`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeProviderConfig(t, tc.body)
			_, err := ValidateProviderFile(path)
			if err == nil || !strings.Contains(err.Error(), "unsupported template variable") {
				t.Fatalf("ValidateProviderFile err=%v, want unsupported template variable", err)
			}
		})
	}
}

func TestValidateProviderFile_NonExpressionFieldsDoNotValidateTemplates(t *testing.T) {
	path := writeProviderConfig(t, `
request {
  set_header "anthropic-beta" "context-1m-2025-08-07";
  filter_header_values "anthropic-beta" "${request.unknown}";
}
models {
  models_mode custom;
  path "/v1/models";
  id_path "$.data[*].id";
  id_regex "^models/.*$";
}
`)
	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}

func TestValidateProviderFile_HeaderNameAndQueryKeyKeepTemplateLiterals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "template-literals.conf")
	content := `syntax "next-router/0.1";

provider "template-literals" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
    request {
      set_header "${request.unknown}" "literal";
      filter_header_values "${request.unknown}" "${request.unknown}";
    }
  }
  match api = "chat.completions" {
    upstream {
      set_path "/v1/chat/completions";
      set_query "${request.unknown}" "literal";
    }
  }
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if len(pf.Headers.Defaults.Request) < 2 {
		t.Fatalf("request header ops=%d, want at least 2", len(pf.Headers.Defaults.Request))
	}
	if got, want := pf.Headers.Defaults.Request[0].NameExpr, `"${request.unknown}"`; got != want {
		t.Fatalf("set_header name expr=%q want %q", got, want)
	}
	if got, want := pf.Headers.Defaults.Request[1].NameExpr, `"${request.unknown}"`; got != want {
		t.Fatalf("filter_header_values name expr=%q want %q", got, want)
	}
	if len(pf.Routing.Matches) != 1 {
		t.Fatalf("routing matches=%d want 1", len(pf.Routing.Matches))
	}
	if _, ok := pf.Routing.Matches[0].QueryPairs["${request.unknown}"]; !ok {
		t.Fatalf("query key was not preserved literally: %#v", pf.Routing.Matches[0].QueryPairs)
	}
}

func TestValidateProviderFile_KeepHeaderValuesParsedAndValidated(t *testing.T) {
	path := writeProviderConfig(t, `
request {
  keep_header_values "anthropic-beta" "computer-use-*" "context-management-*";
  keep_header_values "x-feature-flags" "stable-*" separator=";";
}
`)
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if len(pf.Headers.Defaults.Request) != 2 {
		t.Fatalf("request header ops=%d, want 2", len(pf.Headers.Defaults.Request))
	}
	op := pf.Headers.Defaults.Request[0]
	if op.Op != "header_keep_values" {
		t.Fatalf("op.Op=%q want header_keep_values", op.Op)
	}
	if len(op.Patterns) != 2 || op.Patterns[0] != "computer-use-*" || op.Patterns[1] != "context-management-*" {
		t.Fatalf("unexpected patterns: %#v", op.Patterns)
	}
	op2 := pf.Headers.Defaults.Request[1]
	if op2.Separator != ";" {
		t.Fatalf("separator=%q want ;", op2.Separator)
	}
}

func writeProviderConfig(t *testing.T, defaultsBody string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "template.conf")
	content := `syntax "next-router/0.1";

provider "template" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
` + defaultsBody + `
  }
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
