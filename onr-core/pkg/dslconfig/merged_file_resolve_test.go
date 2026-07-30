package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

// The merged-file loader (ReloadFromFile, used for a single .conf entry point
// with includes) and the per-file loader (ReloadFromDir) build ProviderFile
// through different functions. Only the per-file one used to run the request
// transform resolver, so a merged-file load returned validation rules whose
// compiled path parts were nil and the first request panicked indexing them.
//
// These tests pin the resolved state for the merged path specifically. Loading
// successfully is not enough to catch the regression: the gap only shows when
// something reads the compiled fields.
const mergedResolveProviderConf = `syntax "next-router/0.1";

provider "merged-resolve" {
  defaults {
    upstream_config { base_url = "https://example.invalid"; }
    response { resp_passthrough; }
  }

  match api = "images.generations" {
    request {
      req_required body "$.prompt";
      req_len body "$.prompt" max=1500;
      req_range body "$.n" min=1 max=9;
      req_enum body "$.response_format" "url" "b64_json";
    }
    upstream { set_path "/v1/image_generation"; }
  }
}
`

func mustLoadMergedProviderFile(t *testing.T) ProviderFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), "merged.conf")
	if err := os.WriteFile(path, []byte(mergedResolveProviderConf), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	reg := NewRegistry()
	if _, err := reg.ReloadFromFile(path); err != nil {
		t.Fatalf("ReloadFromFile: %v", err)
	}
	pf, ok := reg.GetProvider("merged-resolve")
	if !ok {
		t.Fatalf("provider not registered")
	}
	return pf
}

func TestMergedFileLoad_ResolvesRequestValidationRules(t *testing.T) {
	pf := mustLoadMergedProviderFile(t)
	meta := &dslmeta.Meta{API: "images.generations", IsStream: false}
	tr, ok := pf.Request.Select(meta)
	if !ok {
		t.Fatalf("no request transform selected")
	}
	if len(tr.ValidationRules) != 4 {
		t.Fatalf("rules got %d want 4", len(tr.ValidationRules))
	}
	for _, rule := range tr.ValidationRules {
		// PathParts is what the runtime indexes into; a nil value is the
		// unresolved state that panicked.
		if len(rule.PathParts) == 0 {
			t.Fatalf("rule %s %s has unresolved PathParts", rule.Op, rule.Path)
		}
	}
}

// req_enum additionally pre-parses its candidates; an unresolved rule would
// accept nothing.
func TestMergedFileLoad_ResolvesEnumCandidates(t *testing.T) {
	pf := mustLoadMergedProviderFile(t)
	meta := &dslmeta.Meta{API: "images.generations", IsStream: false}
	tr, _ := pf.Request.Select(meta)
	for _, rule := range tr.ValidationRules {
		if rule.Op != ReqRuleEnum {
			continue
		}
		if len(rule.LiteralValues) != 2 {
			t.Fatalf("enum literals got %d want 2 (%#v)", len(rule.LiteralValues), rule.LiteralValues)
		}
		return
	}
	t.Fatal("no req_enum rule found")
}

// The merged path also skipped response validation entirely, so a response
// directive the per-file path rejects loaded without complaint. An unsupported
// sse_collect mode is one such case — unlike a resp_map mode, which both paths
// deliberately tolerate and degrade to passthrough at runtime.
func TestMergedFileLoad_ValidatesResponse(t *testing.T) {
	const conf = `syntax "next-router/0.1";

provider "merged-bad-response" {
  defaults {
    upstream_config { base_url = "https://example.invalid"; }
  }

  match api = "chat.completions" stream = false {
    response { sse_collect definitely_not_a_real_mode; }
    upstream { set_path "/v1/chat/completions"; }
  }
}
`
	path := filepath.Join(t.TempDir(), "merged.conf")
	if err := os.WriteFile(path, []byte(conf), 0o600); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	reg := NewRegistry()
	if _, err := reg.ReloadFromFile(path); err == nil {
		t.Fatal("expected an unsupported sse_collect mode to be rejected on the merged path")
	}
}
