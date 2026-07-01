package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dsllang"
)

const validOpenAIConf = `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth { auth_bearer; }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`

const validOpenAIConfUpdated = `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com/v2"; }
    auth { auth_bearer; }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`

const validOpenAIConfWithUsageMetrics = `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth { auth_bearer; }
    metrics {
      usage_extract custom;
      input_tokens_expr = $.usage.input_tokens;
      output_tokens_expr = $.usage.output_tokens;
    }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`

func TestSaveProviderRequiresValidationSuccess(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(dir, "openai.conf")
	if err := os.WriteFile(target, []byte(validOpenAIConf), 0o600); err != nil {
		t.Fatalf("write seed provider conf: %v", err)
	}

	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	invalid := strings.ReplaceAll(validOpenAIConfUpdated, `provider "openai"`, `provider "other"`)
	status, body := postJSON(t, httpSrv.URL+"/api/providers/save", providerRequest{
		Provider: "openai",
		Content:  invalid,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("save invalid status=%d body=%v", status, body)
	}

	gotSeed, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read seed file: %v", err)
	}
	if string(gotSeed) != validOpenAIConf {
		t.Fatalf("file changed after invalid save")
	}

	status, body = postJSON(t, httpSrv.URL+"/api/providers/save", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConfUpdated,
	})
	if status != http.StatusOK {
		t.Fatalf("save valid status=%d body=%v", status, body)
	}

	gotSaved, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(gotSaved) != validOpenAIConfUpdated {
		t.Fatalf("unexpected saved content")
	}
}

func TestProviderEndpoints(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.conf"), []byte(validOpenAIConf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	listResp, err := http.Get(httpSrv.URL + "/api/providers")
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listBody providerResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if !listBody.OK || len(listBody.Providers) != 1 || listBody.Providers[0] != "openai" {
		t.Fatalf("unexpected list body: %+v", listBody)
	}

	getResp, err := http.Get(httpSrv.URL + "/api/provider?name=openai")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d", getResp.StatusCode)
	}
	var getBody providerResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get body: %v", err)
	}
	if !getBody.OK || getBody.Provider != "openai" || strings.TrimSpace(getBody.Content) == "" {
		t.Fatalf("unexpected get body: %+v", getBody)
	}

	status, body := postJSON(t, httpSrv.URL+"/api/providers/validate", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConfUpdated,
	})
	if status != http.StatusOK {
		t.Fatalf("validate status=%d body=%v", status, body)
	}

	status, body = postJSON(t, httpSrv.URL+"/api/providers/validate", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConfWithUsageMetrics,
	})
	if status != http.StatusOK {
		t.Fatalf("validate(metrics) status=%d body=%v", status, body)
	}
	if len(body.Warnings) != 0 {
		t.Fatalf("expected no warnings in validate response, got=%v", body.Warnings)
	}
}

func TestEditorLanguageEndpoints(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.conf"), []byte(validOpenAIConf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}
	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	invalid := strings.Replace(validOpenAIConf, "provider", "provider_bad", 1)
	status, diagBody := postEditorDiagnosticsJSON(t, httpSrv.URL+"/api/editor/diagnostics", editorRequest{
		Provider: "openai",
		Content:  invalid,
	})
	if status != http.StatusOK {
		t.Fatalf("diagnostics status=%d body=%+v", status, diagBody)
	}
	if !diagBody.OK || diagBody.Provider != "openai" || diagBody.TargetFile == "" || diagBody.URI == "" {
		t.Fatalf("unexpected diagnostics body: %+v", diagBody)
	}
	if len(diagBody.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics for invalid content")
	}

	status, tokenBody := postEditorSemanticTokensJSON(t, httpSrv.URL+"/api/editor/semantic-tokens", editorRequest{
		Provider: "openai",
		Content:  validOpenAIConf,
	})
	if status != http.StatusOK {
		t.Fatalf("semantic tokens status=%d body=%+v", status, tokenBody)
	}
	if !tokenBody.OK || len(tokenBody.Legend.TokenTypes) == 0 || len(tokenBody.Tokens.Data) == 0 {
		t.Fatalf("unexpected semantic tokens body: %+v", tokenBody)
	}

	status, hoverBody := postEditorHoverJSON(t, httpSrv.URL+"/api/editor/hover", editorHoverRequest{
		Provider: "openai",
		Content:  validOpenAIConf,
		Position: dsllang.Position{
			Line:      10,
			Character: 6,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("hover status=%d body=%+v", status, hoverBody)
	}
	if !hoverBody.OK || hoverBody.Hover == nil {
		t.Fatalf("unexpected hover body: %+v", hoverBody)
	}
	if hoverBody.Hover.Word != "upstream" || !strings.Contains(hoverBody.Hover.Contents.Value, "Upstream path/query routing") {
		t.Fatalf("unexpected hover contents: %+v", hoverBody.Hover)
	}

	status, emptyHoverBody := postEditorHoverJSON(t, httpSrv.URL+"/api/editor/hover", editorHoverRequest{
		Provider: "openai",
		Content:  validOpenAIConf,
		Position: dsllang.Position{
			Line:      10,
			Character: 0,
		},
	})
	if status != http.StatusOK {
		t.Fatalf("empty hover status=%d body=%+v", status, emptyHoverBody)
	}
	if !emptyHoverBody.OK || emptyHoverBody.Hover != nil {
		t.Fatalf("expected empty hover, got: %+v", emptyHoverBody)
	}

	status, formatBody := postEditorFormatJSON(t, httpSrv.URL+"/api/editor/format", editorRequest{
		Provider: "openai",
		Content:  `provider "openai" { defaults { auth { auth_bearer; } } }`,
	})
	if status != http.StatusOK {
		t.Fatalf("format status=%d body=%+v", status, formatBody)
	}
	if !formatBody.OK || !strings.Contains(formatBody.Content, "\n  defaults {") {
		t.Fatalf("unexpected format body: %+v", formatBody)
	}
}

func TestProviderEndpoints_IgnoreGlobalUsageModeFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usage-modes.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_openai" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
`), 0o600); err != nil {
		t.Fatalf("write usage mode conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.conf"), []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth { auth_bearer; }
    metrics { usage_extract shared_openai; }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}

	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	listResp, err := http.Get(httpSrv.URL + "/api/providers")
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listBody providerResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if !listBody.OK || len(listBody.Providers) != 1 || listBody.Providers[0] != "openai" {
		t.Fatalf("unexpected list body: %+v", listBody)
	}
}

func TestProviderValidate_UsesSiblingOnrConfig(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "providers")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "onr.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_openai" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
`), 0o600); err != nil {
		t.Fatalf("write onr.conf: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.conf"), []byte(validOpenAIConf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}

	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	status, body := postJSON(t, httpSrv.URL+"/api/providers/validate", providerRequest{
		Provider: "openai",
		Content: `
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com/v2"; }
    auth { auth_bearer; }
    metrics { usage_extract shared_openai; }
  }

  match api = "chat.completions" {
    upstream { set_path "/v1/chat/completions"; }
    response { resp_passthrough; }
  }
}
`,
	})
	if status != http.StatusOK {
		t.Fatalf("validate status=%d body=%v", status, body)
	}
}

func TestNewServer_AllowsMissingProvidersDirectorySource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "providers")

	srv, err := NewServer(dir)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	listResp, err := http.Get(httpSrv.URL + "/api/providers")
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listBody providerResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if !listBody.OK || len(listBody.Providers) != 0 {
		t.Fatalf("unexpected list body: %+v", listBody)
	}
}

func TestProviderEndpoints_MixedInlineAndIncludedDirSource(t *testing.T) {
	root := t.TempDir()
	providersDir := filepath.Join(root, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providersDir, "anthropic.conf"), []byte(`
syntax "next-router/0.1";

provider "anthropic" {
  defaults {
    upstream_config { base_url = "https://api.anthropic.com"; }
    auth { auth_header_key "x-api-key"; }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}
	sourcePath := filepath.Join(root, "onr.conf")
	if err := os.WriteFile(sourcePath, []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    auth { auth_bearer; }
  }
}

include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv, err := NewServer(sourcePath)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	listResp, err := http.Get(httpSrv.URL + "/api/providers")
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listBody providerResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if !listBody.OK || len(listBody.Providers) != 2 {
		t.Fatalf("unexpected list body: %+v", listBody)
	}

	getResp, err := http.Get(httpSrv.URL + "/api/provider?name=openai")
	if err != nil {
		t.Fatalf("get openai: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get openai status=%d", getResp.StatusCode)
	}

	getResp2, err := http.Get(httpSrv.URL + "/api/provider?name=anthropic")
	if err != nil {
		t.Fatalf("get anthropic: %v", err)
	}
	defer func() { _ = getResp2.Body.Close() }()
	if getResp2.StatusCode != http.StatusOK {
		t.Fatalf("get anthropic status=%d", getResp2.StatusCode)
	}
}

func TestProviderEndpoints_FileSourceWithIncludedProvidersDir(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	providersDir := filepath.Join(configDir, "providers")
	if err := os.MkdirAll(providersDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	onrPath := filepath.Join(configDir, "onr.conf")
	if err := os.WriteFile(onrPath, []byte(`
syntax "next-router/0.1";

include providers/*.conf;
`), 0o600); err != nil {
		t.Fatalf("write onr.conf: %v", err)
	}
	target := filepath.Join(providersDir, "openai.conf")
	if err := os.WriteFile(target, []byte(validOpenAIConf), 0o600); err != nil {
		t.Fatalf("write provider conf: %v", err)
	}

	srv, err := NewServer(onrPath)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	listResp, err := http.Get(httpSrv.URL + "/api/providers")
	if err != nil {
		t.Fatalf("list providers: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listBody providerResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listBody); err != nil {
		t.Fatalf("decode list body: %v", err)
	}
	if !listBody.OK || len(listBody.Providers) != 1 || listBody.Providers[0] != "openai" {
		t.Fatalf("unexpected list body: %+v", listBody)
	}

	getResp, err := http.Get(httpSrv.URL + "/api/provider?name=openai")
	if err != nil {
		t.Fatalf("get provider: %v", err)
	}
	defer func() { _ = getResp.Body.Close() }()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status=%d", getResp.StatusCode)
	}
	var getBody providerResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatalf("decode get body: %v", err)
	}
	if getBody.TargetFile != target {
		t.Fatalf("target file=%q want=%q", getBody.TargetFile, target)
	}

	status, body := postJSON(t, httpSrv.URL+"/api/providers/save", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConfUpdated,
	})
	if status != http.StatusOK {
		t.Fatalf("save status=%d body=%v", status, body)
	}
	saved, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(saved) != validOpenAIConfUpdated {
		t.Fatalf("unexpected saved content")
	}
}

func TestProviderEndpoints_MissingMergedFileSource(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "providers.conf")

	srv, err := NewServer(sourcePath)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	status, body := postJSON(t, httpSrv.URL+"/api/providers/validate", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConf,
	})
	if status != http.StatusOK {
		t.Fatalf("validate status=%d body=%+v", status, body)
	}

	status, body = postJSON(t, httpSrv.URL+"/api/providers/save", providerRequest{
		Provider: "openai",
		Content:  validOpenAIConf,
	})
	if status != http.StatusOK {
		t.Fatalf("save status=%d body=%+v", status, body)
	}

	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read merged source: %v", err)
	}
	if !strings.Contains(string(got), `provider "openai"`) {
		t.Fatalf("unexpected merged source content: %s", string(got))
	}
}

func TestResolveDefaultAPIBaseURL_FromEnv(t *testing.T) {
	t.Setenv(envAPIBaseURL, "https://example.internal:3344/")
	got := resolveDefaultAPIBaseURL()
	if got != "https://example.internal:3344" {
		t.Fatalf("resolveDefaultAPIBaseURL=%q", got)
	}
}

func TestResolveListenAddress_Priority(t *testing.T) {
	t.Setenv(envListen, "127.0.0.1:4411")
	got := resolveListenAddress("127.0.0.1:5511")
	if got != "127.0.0.1:5511" {
		t.Fatalf("resolveListenAddress override=%q", got)
	}
}

func TestResolveListenAddress_FromEnv(t *testing.T) {
	t.Setenv(envListen, "127.0.0.1:4411")
	got := resolveListenAddress("")
	if got != "127.0.0.1:4411" {
		t.Fatalf("resolveListenAddress env=%q", got)
	}
}

func TestResolveListenAddress_Default(t *testing.T) {
	t.Setenv(envListen, "")
	got := resolveListenAddress("")
	if got != defaultListen {
		t.Fatalf("resolveListenAddress default=%q", got)
	}
}

func TestRenderIndexHTML_ReplacesDefaultAPIBaseURL(t *testing.T) {
	out := renderIndexHTML("https://onr.local:3300/")
	if strings.Contains(out, "__ONR_ADMIN_WEB_CURL_API_BASE_URL__") {
		t.Fatalf("placeholder should be replaced")
	}
	if !strings.Contains(out, `value="https://onr.local:3300"`) {
		t.Fatalf("unexpected rendered html")
	}
	if !strings.Contains(out, `href="/app.css"`) || !strings.Contains(out, `src="/app.js"`) {
		t.Fatalf("missing split web assets")
	}
	if !strings.Contains(out, `codemirror@`) {
		t.Fatalf("missing CodeMirror editor assets")
	}
	if !strings.Contains(out, `id="requestIdInput"`) || !strings.Contains(out, `id="loadDumpBtn"`) || !strings.Contains(out, `id="dumpOutput"`) {
		t.Fatalf("missing dump request_id ui elements")
	}
}

func TestStaticAssetEndpoints(t *testing.T) {
	srv, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	cases := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/app.css", contentType: "text/css", contains: ".editor-shell"},
		{path: "/app.js", contentType: "application/javascript", contains: "CodeMirror.fromTextArea"},
	}
	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := http.Get(httpSrv.URL + tt.path)
			if err != nil {
				t.Fatalf("get asset: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status=%d", resp.StatusCode)
			}
			if got := resp.Header.Get("Content-Type"); !strings.Contains(got, tt.contentType) {
				t.Fatalf("content-type=%q want %q", got, tt.contentType)
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			if !strings.Contains(string(body), tt.contains) {
				t.Fatalf("asset body missing %q", tt.contains)
			}
		})
	}
}

func TestDumpByRequestIDEndpoint(t *testing.T) {
	providersDir := t.TempDir()
	dumpsDir := t.TempDir()
	rid := "rid-web-test-1"
	dumpPath := filepath.Join(dumpsDir, rid+".log")
	content := "=== META ===\nrequest_id=" + rid + "\n\n=== PROXY RESPONSE ===\nstatus=200\n"
	if err := os.WriteFile(dumpPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write dump file: %v", err)
	}

	srv, err := newServer(providersDir, dumpsDir, defaultAPIBaseURL)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + "/api/dumps/by-request-id?request_id=" + rid)
	if err != nil {
		t.Fatalf("get dump by request id: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	var out dumpByRequestIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !out.OK {
		t.Fatalf("unexpected body: %+v", out)
	}
	if out.RequestID != rid {
		t.Fatalf("request_id mismatch: %q", out.RequestID)
	}
	if out.Path != dumpPath {
		t.Fatalf("path mismatch: %q", out.Path)
	}
	if !strings.Contains(out.Content, "=== META ===") {
		t.Fatalf("content mismatch: %+v", out)
	}
}

func TestDumpByRequestIDEndpoint_BadRequest(t *testing.T) {
	srv, err := newServer(t.TempDir(), t.TempDir(), defaultAPIBaseURL)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + "/api/dumps/by-request-id")
	if err != nil {
		t.Fatalf("get dump by request id: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestDumpByRequestIDEndpoint_NotFound(t *testing.T) {
	srv, err := newServer(t.TempDir(), t.TempDir(), defaultAPIBaseURL)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	resp, err := http.Get(httpSrv.URL + "/api/dumps/by-request-id?request_id=rid-not-found")
	if err != nil {
		t.Fatalf("get dump by request id: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestDumpByRequestIDEndpoint_MethodNotAllowed(t *testing.T) {
	srv, err := newServer(t.TempDir(), t.TempDir(), defaultAPIBaseURL)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	req, err := http.NewRequest(http.MethodPost, httpSrv.URL+"/api/dumps/by-request-id", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post dump by request id: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", resp.StatusCode)
	}
}

func TestTestRequestEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if got := strings.TrimSpace(r.Header.Get("Authorization")); got == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		if got := strings.TrimSpace(r.Header.Get("x-onr-provider")); got != "openai" {
			http.Error(w, "missing provider", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	srv, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	status, body := postTestJSON(t, httpSrv.URL+"/api/test/request", testRequest{
		BaseURL:       upstream.URL,
		Path:          "/v1/chat/completions",
		Authorization: "Bearer onr:v1?k=change-me&p=openai&m=gpt-4o-mini",
		Provider:      "openai",
		Payload:       `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`,
	})
	if status != http.StatusOK {
		t.Fatalf("status=%d body=%+v", status, body)
	}
	if !body.OK || body.Status != http.StatusOK {
		t.Fatalf("unexpected body: %+v", body)
	}
	if !strings.Contains(body.Body, `"ok":true`) {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func TestTestRequestEndpoint_InvalidPayload(t *testing.T) {
	srv, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	httpSrv := httptest.NewServer(srv.Handler())
	defer httpSrv.Close()

	status, body := postTestJSON(t, httpSrv.URL+"/api/test/request", testRequest{
		BaseURL:       "http://127.0.0.1:3300",
		Path:          "/v1/chat/completions",
		Authorization: "Bearer onr:v1?k=change-me&p=openai&m=gpt-4o-mini",
		Provider:      "openai",
		Payload:       "not-json",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status=%d body=%+v", status, body)
	}
	if body.OK || !strings.Contains(body.Error, "payload must be valid json") {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func postJSON(t *testing.T, url string, body any) (int, providerResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out providerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func postTestJSON(t *testing.T, url string, body any) (int, testResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out testResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func postEditorDiagnosticsJSON(t *testing.T, url string, body any) (int, editorDiagnosticsResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out editorDiagnosticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func postEditorSemanticTokensJSON(t *testing.T, url string, body any) (int, editorSemanticTokensResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out editorSemanticTokensResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func postEditorHoverJSON(t *testing.T, url string, body any) (int, editorHoverResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out editorHoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}

func postEditorFormatJSON(t *testing.T, url string, body any) (int, editorFormatResponse) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("post %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out editorFormatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, out
}
