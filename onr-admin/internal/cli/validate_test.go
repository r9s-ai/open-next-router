package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProviders_DefaultOutput(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := validateProviders(
		"../../../config/providers",
		false,
		&out,
	)
	if err != nil {
		t.Fatalf("validateProviders: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "validate providers: OK") {
		t.Fatalf("expected success output, got=%q", got)
	}
	if strings.Contains(got, "\"usage\"") {
		t.Fatalf("did not expect usage plan json in default output, got=%q", got)
	}
}

func TestValidateProviders_ShowUsagePlan(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := validateProviders(
		"../../../config/providers",
		true,
		&out,
	)
	if err != nil {
		t.Fatalf("validateProviders: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "validate providers: OK") {
		t.Fatalf("expected success output, got=%q", got)
	}
	if !strings.Contains(got, "\"provider\": \"openai\"") {
		t.Fatalf("expected openai usage plan in output, got=%q", got)
	}
	if !strings.Contains(got, "\"audio.tts\"") {
		t.Fatalf("expected compiled audio.tts fact in output, got=%q", got)
	}
	if !strings.Contains(got, "\"$.audio_duration_seconds\"") {
		t.Fatalf("expected derived audio duration path in output, got=%q", got)
	}
}

func TestValidateProviders_ShowUsagePlan_WithGlobalUsageMode(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "usage-modes.conf"), []byte(`
syntax "next-router/0.1";

usage_mode "shared_tokens" {
  usage_extract custom;
  usage_fact input token path="$.usage.input_tokens";
  usage_fact output token path="$.usage.output_tokens";
}
`), 0o600); err != nil {
		t.Fatalf("write usage mode file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "openai.conf"), []byte(`
syntax "next-router/0.1";

provider "openai" {
  defaults {
    upstream_config { base_url = "https://api.openai.com"; }
    metrics { usage_extract shared_tokens; }
  }
}
`), 0o600); err != nil {
		t.Fatalf("write provider file: %v", err)
	}

	var out bytes.Buffer
	err := validateProviders(dir, true, &out)
	if err != nil {
		t.Fatalf("validateProviders: %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "validate providers: OK") {
		t.Fatalf("expected success output, got=%q", got)
	}
	if !strings.Contains(got, "\"provider\": \"openai\"") {
		t.Fatalf("expected openai usage plan in output, got=%q", got)
	}
	if !strings.Contains(got, "\"$.usage.input_tokens\"") || !strings.Contains(got, "\"$.usage.output_tokens\"") {
		t.Fatalf("expected compiled global usage mode facts in output, got=%q", got)
	}
}
