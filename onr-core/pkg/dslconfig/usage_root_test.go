package dslconfig

import (
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestLastScalarPathKey(t *testing.T) {
	for path, want := range map[string]string{
		"$.duration":                   "duration",
		"$.parameters.durationSeconds": "durationSeconds",
		"$.parameters.numberOfVideos":  "numberOfVideos",
		"$.parameters.resolution":      "resolution",
		"$.a.b.c":                      "c",
		"$.foo[0]":                     "foo", // bracket stripped, segment before it
		"$.foo[0].bar":                 "bar", // last dotted segment
		"$":                            "",    // root path has no key
		"duration":                     "",    // missing $. prefix
		"":                             "",    // empty
		"  $.padded.key  ":             "key", // trimmed
	} {
		if got := lastScalarPathKey(path); got != want {
			t.Errorf("lastScalarPathKey(%q) = %q, want %q", path, got, want)
		}
	}
}

// source=request reads from the request body, not the response, even when
// the same path exists in both.
func TestExtractUsageRoot_ScalarSourceRequest(t *testing.T) {
	roots := []usageRootConfig{
		{Path: "$.parameters.durationSeconds", Source: "request"},
	}
	reqRoot := map[string]any{
		"parameters": map[string]any{"durationSeconds": 5.0},
	}
	respRoot := map[string]any{
		"parameters": map[string]any{"durationSeconds": 99.0},
	}
	got := extractUsageRootWithEvent("", reqRoot, respRoot, roots)
	if got == nil {
		t.Fatal("expected non-nil merged root")
	}
	if v, _ := got["durationSeconds"].(float64); v != 5.0 {
		t.Errorf("durationSeconds = %v, want 5.0 from request", got["durationSeconds"])
	}
}

// Multiple scalar source=request rules each contribute their own key.
func TestExtractUsageRoot_MultipleScalarSourceRequest(t *testing.T) {
	roots := []usageRootConfig{
		{Path: "$.parameters.durationSeconds", Source: "request"},
		{Path: "$.parameters.resolution", Source: "request"},
		{Path: "$.parameters.numberOfVideos", Source: "request"},
	}
	reqRoot := map[string]any{
		"parameters": map[string]any{
			"durationSeconds": 5.0,
			"resolution":      "1080p",
			"numberOfVideos":  float64(2),
		},
	}
	got := extractUsageRootWithEvent("", reqRoot, nil, roots)
	if got == nil {
		t.Fatal("expected non-nil merged root")
	}
	if v, _ := got["durationSeconds"].(float64); v != 5.0 {
		t.Errorf("durationSeconds = %v, want 5.0", got["durationSeconds"])
	}
	if v, _ := got["resolution"].(string); v != "1080p" {
		t.Errorf("resolution = %v, want 1080p", got["resolution"])
	}
	if v, _ := got["numberOfVideos"].(float64); v != 2.0 {
		t.Errorf("numberOfVideos = %v, want 2", got["numberOfVideos"])
	}
}

// Without source=request the default source is the response body.
func TestExtractUsageRoot_DefaultSourceReadsResponse(t *testing.T) {
	roots := []usageRootConfig{
		{Path: "$.parameters.durationSeconds"},
	}
	reqRoot := map[string]any{
		"parameters": map[string]any{"durationSeconds": 5.0},
	}
	respRoot := map[string]any{
		"parameters": map[string]any{"durationSeconds": 8.0},
	}
	got := extractUsageRootWithEvent("", reqRoot, respRoot, roots)
	if got == nil {
		t.Fatal("expected non-nil merged root")
	}
	if v, _ := got["durationSeconds"].(float64); v != 8.0 {
		t.Errorf("durationSeconds = %v, want 8.0 from response", got["durationSeconds"])
	}
}

// When the request root is nil, source=request rules contribute nothing.
func TestExtractUsageRoot_MissingRequestRoot(t *testing.T) {
	roots := []usageRootConfig{
		{Path: "$.parameters.durationSeconds", Source: "request"},
	}
	got := extractUsageRootWithEvent("", nil, nil, roots)
	if len(got) != 0 {
		t.Errorf("expected empty root when reqRoot is nil, got %v", got)
	}
}

// Integration via ExtractUsage: usage.UsageRoot carries scalar source=request values
// extracted from meta.RequestBody, not from the response body.
func TestExtractUsage_UsageRootScalarSourceRequest(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.parameters.durationSeconds", Source: "request"},
			{Path: "$.parameters.numberOfVideos", Source: "request"},
		},
	}
	meta := &dslmeta.Meta{
		RequestBody:        []byte(`{"parameters":{"durationSeconds":7.0,"numberOfVideos":3}}`),
		RequestContentType: "application/json",
	}
	usage, _, err := ExtractUsage(meta, &cfg, []byte(`{}`))
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage")
	}
	if v, _ := usage.UsageRoot["durationSeconds"].(float64); v != 7.0 {
		t.Errorf("UsageRoot.durationSeconds = %v, want 7.0", usage.UsageRoot["durationSeconds"])
	}
	if v, _ := usage.UsageRoot["numberOfVideos"].(float64); v != 3.0 {
		t.Errorf("UsageRoot.numberOfVideos = %v, want 3", usage.UsageRoot["numberOfVideos"])
	}
}

// Response body values must not bleed into source=request roots even when
// the response happens to contain fields at the same path.
func TestExtractUsage_UsageRootSourceRequestIgnoresResponse(t *testing.T) {
	cfg := UsageExtractConfig{
		Mode: usageModeCustom,
		usageRoots: []usageRootConfig{
			{Path: "$.parameters.durationSeconds", Source: "request"},
		},
	}
	meta := &dslmeta.Meta{
		RequestBody:        []byte(`{"parameters":{"durationSeconds":5.0}}`),
		RequestContentType: "application/json",
	}
	// Response also contains the same path with a different value.
	usage, _, err := ExtractUsage(meta, &cfg, []byte(`{"parameters":{"durationSeconds":99.0}}`))
	if err != nil {
		t.Fatalf("ExtractUsage: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage")
	}
	if v, _ := usage.UsageRoot["durationSeconds"].(float64); v != 5.0 {
		t.Errorf("UsageRoot.durationSeconds = %v, want 5.0 from request (not 99 from response)", usage.UsageRoot["durationSeconds"])
	}
}

// DSL round-trip: ParseUsageMode correctly parses usage_root with source=request.
func TestParseUsageMode_UsageRootSourceRequest(t *testing.T) {
	cfg, err := ParseUsageMode("usage_modes.conf", `
usage_mode "veo_params" {
  usage_root path="$.parameters.durationSeconds" source=request;
  usage_root path="$.parameters.resolution" source=request;
  usage_root path="$.parameters.numberOfVideos" source=request;
}
`, "veo_params")
	if err != nil {
		t.Fatalf("ParseUsageMode: %v", err)
	}
	plan := cfg.CompiledPlan(nil)
	if len(plan.UsageRoots) != 3 {
		t.Fatalf("UsageRoots count = %d, want 3", len(plan.UsageRoots))
	}
	wantPaths := map[string]bool{
		"$.parameters.durationSeconds": true,
		"$.parameters.resolution":      true,
		"$.parameters.numberOfVideos":  true,
	}
	for _, root := range plan.UsageRoots {
		if root.Source != "request" {
			t.Errorf("UsageRoot %q: Source = %q, want %q", root.Path, root.Source, "request")
		}
		if !wantPaths[root.Path] {
			t.Errorf("unexpected path %q in UsageRoots", root.Path)
		}
	}
}
