package usageextract

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
)

const anthropicUsageMode = `
usage_mode "anthropic_messages" {
  usage_root path="$.usage";

  usage_fact input token path="$.input_tokens";
  usage_fact input token path="$.cache_read_input_tokens";
  usage_fact input token path="$.cache_creation_input_tokens";
  usage_fact output token path="$.output_tokens";
  usage_fact cache_read token path="$.cache_read_input_tokens";
  usage_fact cache_write token path="$.cache_creation.ephemeral_5m_input_tokens" attr.ttl="5m";
  usage_fact cache_write token path="$.cache_creation_input_tokens" fallback=true;
  usage_fact server_tool.web_search call path="$.server_tool_use.web_search_requests";
}
`

func TestRunCLIExtractsFactsFromInlineUsageMode(t *testing.T) {
	t.Parallel()

	stdout, stderr, code := runCLI(t, []string{
		"--usage-mode-dsl", anthropicUsageMode,
		"--usage", `{"usage":{"input_tokens":10,"output_tokens":4,"cache_read_input_tokens":2,"cache_creation":{"ephemeral_5m_input_tokens":3},"cache_creation_input_tokens":3,"server_tool_use":{"web_search_requests":1}}}`,
	}, "")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	facts := decodeFacts(t, stdout)
	if !strings.Contains(stdout, "\n  {") {
		t.Fatalf("expected formatted JSON output, got %q", stdout)
	}
	assertFact(t, facts, "input", "token", 10, nil)
	assertFact(t, facts, "input", "token", 2, nil)
	assertFact(t, facts, "input", "token", 3, nil)
	assertFact(t, facts, "output", "token", 4, nil)
	assertFact(t, facts, "cache_read", "token", 2, nil)
	assertFact(t, facts, "cache_write", "token", 3, map[string]string{"ttl": "5m"})
	assertFact(t, facts, "server_tool.web_search", "call", 1, nil)
}

func TestRunCLISelectsModeFromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "usage_modes.conf")
	content := `
usage_mode "first" { usage_fact input token path="$.first"; }
usage_mode "second" { usage_fact output token path="$.second"; }
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write mode file: %v", err)
	}
	stdout, stderr, code := runCLI(t, []string{
		"--usage-mode-file", path,
		"--usage-mode", "second",
		"--usage", `{"first":1,"second":2}`,
	}, "")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	facts := decodeFacts(t, stdout)
	if len(facts) != 1 {
		t.Fatalf("facts=%#v", facts)
	}
	assertFact(t, facts, "output", "token", 2, nil)
}

func TestRunCLIExtractsFactsFromSSE(t *testing.T) {
	t.Parallel()

	mode := `
usage_mode "anthropic_stream" {
  usage_root path="$.message.usage" event="message_start" event_optional=true exclude="output_tokens";
  usage_root path="$.usage" event="message_delta" event_optional=true;
  usage_fact input token path="$.input_tokens";
  usage_fact output token path="$.output_tokens";
  usage_fact cache_read token path="$.cache_read_input_tokens";
}
`
	sse := "event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":3,\ndata: \"cache_read_input_tokens\":2}}}\n\nevent: message_delta\ndata: {\"usage\":{\"output_tokens\":7}}\n\ndata: [DONE]\n\n"
	stdout, stderr, code := runCLI(t, []string{
		"--stream",
		"--usage-mode-dsl", mode,
	}, sse)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	facts := decodeFacts(t, stdout)
	assertFact(t, facts, "input", "token", 3, nil)
	assertFact(t, facts, "output", "token", 7, nil)
	assertFact(t, facts, "cache_read", "token", 2, nil)
}

func TestRunCLIUsesRequestAndDerivedContext(t *testing.T) {
	t.Parallel()

	mode := `
usage_mode "context" {
  usage_fact input token source=request path="$.request_tokens";
  usage_fact audio.tts second source=derived path="$.seconds";
}
`
	stdout, stderr, code := runCLI(t, []string{
		"--usage-mode-dsl", mode,
		"--usage", `{}`,
		"--request", `{"request_tokens":5}`,
		"--derived", `{"seconds":1.5}`,
	}, "")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	facts := decodeFacts(t, stdout)
	assertFact(t, facts, "input", "token", 5, nil)
	assertFact(t, facts, "audio.tts", "second", 1.5, nil)
}

func TestRunCLIRejectsInvalidInputsAndReturnsEmptyFacts(t *testing.T) {
	t.Parallel()

	mode := `usage_mode "one" { usage_fact input token path="$.input_tokens"; }`
	stdout, stderr, code := runCLI(t, []string{
		"--usage-mode-dsl", mode,
		"--usage", `{}`,
	}, "")
	if code != 0 || stderr != "" || strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	_, stderr, code = runCLI(t, []string{
		"--usage-mode-dsl", mode,
		"--usage", `{`,
	}, "")
	if code != 1 || !strings.Contains(stderr, "parse response json") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}

	_, stderr, code = runCLI(t, []string{
		"--usage-mode-dsl", mode,
		"--usage-mode-file", "also.conf",
		"--usage", `{}`,
	}, "")
	if code != 2 || !strings.Contains(stderr, "cannot be used together") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func runCLI(t *testing.T, args []string, stdin string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := RunCLI(args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func decodeFacts(t *testing.T, output string) []dslconfig.UsageFact {
	t.Helper()
	var facts []dslconfig.UsageFact
	if err := json.Unmarshal([]byte(output), &facts); err != nil {
		t.Fatalf("decode facts %q: %v", output, err)
	}
	return facts
}

func assertFact(t *testing.T, facts []dslconfig.UsageFact, dimension, unit string, quantity float64, attrs map[string]string) {
	t.Helper()
	for _, fact := range facts {
		if fact.Dimension != dimension || fact.Unit != unit || fact.Quantity != quantity {
			continue
		}
		if len(attrs) != len(fact.Attributes) {
			continue
		}
		matched := true
		for key, value := range attrs {
			if fact.Attributes[key] != value {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("missing fact dimension=%q unit=%q quantity=%v attrs=%#v in %#v", dimension, unit, quantity, attrs, facts)
}
