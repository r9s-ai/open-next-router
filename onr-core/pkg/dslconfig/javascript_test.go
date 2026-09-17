package dslconfig

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsext"
)

func TestJSSlotsAndAtomicReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.conf")
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := `provider "p" { defaults { upstream_config {base_url = "https://example.com";} js_timeout 300ms; request {request_by_js_block {ctx.state.before=true;} after_req_map {request_by_js_block {ctx.state.after=true;}}} } match api=" chat.completions " {request {request_by_js off;} upstream {path="/v1/chat/completions";} } }`
	write(source)
	r := NewRegistry()
	if err := r.ConfigureJS(jsext.RuntimeConfig{}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReloadFromDir(dir); err != nil {
		t.Fatal(err)
	}
	pf, _ := r.GetProvider("p")
	cfg := pf.JS.Select("chat.completions", false)
	if cfg.Has(jsext.Request) || !cfg.Has(jsext.AfterRequestMap) || cfg.Limits.Timeout.Milliseconds() != 300 {
		t.Fatal(cfg)
	}
	write(`provider "p" { defaults {request {request_by_js_block {broken( }}}}`)
	if _, err := r.ReloadFromDir(dir); err == nil {
		t.Fatal("bad reload published")
	}
	still, _ := r.GetProvider("p")
	if !still.JS.Select("chat.completions", false).Has(jsext.AfterRequestMap) {
		t.Fatal("policy lost")
	}
	write(source)
	if _, err := r.ReloadFromFile(path); err != nil {
		t.Fatal(err)
	}
}
func TestJSValidation(t *testing.T) {
	for _, fragment := range []string{
		`request {request_by_js_block {} request_by_js off;}`,
		`request {request_by_js_block {}} request {request_by_js_block {}}`,
		`request {after_req_map {request_by_js_block {}} after_req_map {request_by_js off;}}`,
		`response {request_by_js_block {}}`, `request {js_timeout 1s;}`, `js_timeout 0ms;`, `js_event_limit -1;`, `request {request_after_map_by_js_block {}}`,
	} {
		source := `provider "p" {defaults {` + fragment + `}}`
		if _, err := parseProviderJS("p.conf", source, "p"); err == nil {
			t.Fatalf("accepted %s", fragment)
		}
	}
}
func TestJSIncludeOpaqueAndFileSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.conf")
	script := filepath.Join(dir, "policy.js")
	if err := os.WriteFile(script, []byte(`ctx.state.version=1;`), 0600); err != nil {
		t.Fatal(err)
	}
	source := `provider "p" {defaults {upstream_config {base_url="https://example.com";} request {request_by_js_file policy.js;after_req_map { request_by_js_block {const include="missing"; const regex=/}/; const t=` + "`include x; ${1}`" + `;}}}}}`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	if err := r.ConfigureJS(jsext.RuntimeConfig{Root: dir}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReloadFromDir(dir); err != nil {
		t.Fatal(err)
	}
	old, _ := r.GetProvider("p")
	if err := os.WriteFile(script, []byte(`ctx.reject(403,{error:"new"});`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReloadFromDir(dir); err != nil {
		t.Fatal(err)
	}
	s, err := jsext.NewSession(old.JS.Select("a", false), jsext.Meta{}, old.JS.HTTP, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Run(context.Background(), jsext.Request, &jsext.Message{ContentType: "application/json", Body: `{}`}); err != nil {
		t.Fatal("old snapshot changed", err)
	}
}

func TestJSBundleAndOriginalSourceDiagnostics(t *testing.T) {
	dir := t.TempDir()
	fragment := filepath.Join(dir, "request.inc")
	root := filepath.Join(dir, "p.conf")
	body := "{\n const pattern=/[{}]/;\n const text=`include absent; ${ {x:1}.x }`;\n ctx.state.text=text;\n}"
	source := `provider "p" {defaults {upstream_config {base_url="https://example.com";} request {include "request.inc";}}}`
	if err := os.WriteFile(fragment, []byte("\nrequest_by_js_block "+body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	bundle, err := BundleProvidersPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bundle, body) {
		t.Fatalf("JS changed by bundle: %s", bundle)
	}
	output := filepath.Join(dir, "merged.conf")
	if err := os.WriteFile(output, []byte(bundle), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProvidersFile(output); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fragment, []byte("\nrequest_by_js_block {\n const ok=1;\n const = 2;\n}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProviderFile(root); err == nil || !strings.Contains(err.Error(), "request.inc:4:") {
		t.Fatalf("original syntax location lost: %v", err)
	}
	if err := os.WriteFile(fragment, []byte("\nrequest_by_js_block {\n with({}) {}\n}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateProviderFile(root); err == nil || !strings.Contains(err.Error(), "request.inc:3:") {
		t.Fatalf("strict compilation location lost: %v", err)
	}
}

func TestJSAuthorizationPublishedAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.conf")
	source := `provider "p" {defaults {upstream_config {base_url="https://example.com";} request {request_by_js_block {return;}}}}`
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	old := jsext.RuntimeConfig{HTTP: map[string]jsext.HTTPPolicy{"p": {Enabled: true, AllowedOrigins: []string{"https://old.example"}}}}
	if _, err := r.ReloadFromPathWithJS(path, old); err != nil {
		t.Fatal(err)
	}
	snapshot := r.Snapshot()
	next := jsext.RuntimeConfig{HTTP: map[string]jsext.HTTPPolicy{"p": {Enabled: true, AllowedOrigins: []string{"https://new.example"}}}}
	if err := os.WriteFile(path, []byte(`provider "p" {`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReloadFromPathWithJS(path, next); err == nil {
		t.Fatal("invalid transaction committed")
	}
	before, _ := r.GetProvider("p")
	if before.JS.HTTP.AllowedOrigins[0] != "https://old.example" {
		t.Fatal("authorization partially published")
	}
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReloadFromPathWithJS(path, next); err != nil {
		t.Fatal(err)
	}
	pinned, _ := snapshot.GetProvider("p")
	current, _ := r.GetProvider("p")
	if pinned.JS.HTTP.AllowedOrigins[0] != "https://old.example" || current.JS.HTTP.AllowedOrigins[0] != "https://new.example" {
		t.Fatal("mixed snapshots")
	}
}
