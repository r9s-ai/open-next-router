package dslconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeObservabilityProvider(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "example.conf")
	content := `provider "example" {
	defaults { upstream_config { base_url = "https://example.com"; } }
` + body + "\n}\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateProviderFile_ObservabilityUpstreamRequestID(t *testing.T) {
	path := writeObservabilityProvider(t, `
	observability {
		upstream_request_id "x-request-id" "OpenAI-Request-ID";
	}
`)
	pf, err := ValidateProviderFile(path)
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if pf.Observability.UpstreamRequestID == nil {
		t.Fatal("expected upstream request ID rule")
	}
	want := []string{"x-request-id", "OpenAI-Request-ID"}
	if got := pf.Observability.UpstreamRequestID.Headers; strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("headers=%v want=%v", got, want)
	}
}

func TestValidateProviderFile_ObservabilityRejectsInvalidRules(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"missing semicolon", `observability { upstream_request_id "x-request-id" }`, "expected ';' after upstream_request_id"},
		{"invalid header", `observability { upstream_request_id "bad header"; }`, "invalid upstream request ID header name"},
		{"duplicate header", `observability { upstream_request_id "X-Request-ID" "x-request-id"; }`, "duplicate upstream request ID header"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateProviderFile(writeObservabilityProvider(t, tt.body))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want substring %q", err, tt.want)
			}
		})
	}
}

func TestValidateProviderFile_ObservabilityRejectsEmptyRule(t *testing.T) {
	_, err := ValidateProviderFile(writeObservabilityProvider(t, `
	observability {
		upstream_request_id;
	}
`))
	if err == nil || !strings.Contains(err.Error(), "requires at least one header name") {
		t.Fatalf("error=%v want missing-header validation error", err)
	}
}

func TestValidateProviderFile_ObservabilityMissingIsUnset(t *testing.T) {
	pf, err := ValidateProviderFile(writeObservabilityProvider(t, ""))
	if err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
	if pf.Observability.UpstreamRequestID != nil {
		t.Fatalf("expected unset rule, got %#v", pf.Observability.UpstreamRequestID)
	}
}
