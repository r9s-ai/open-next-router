package dslconfig

import (
	"strings"
	"testing"
)

func FuzzDSLParser(f *testing.F) {
	for _, seed := range []string{
		`provider "demo" { defaults { upstream_config { base_url = "https://api.example.com"; } } }`,
		`provider "demo" { match api = "/v1/chat/completions" stream = true { upstream { path "/v1/chat/completions"; } } }`,
		`provider "demo" { defaults { request { req_map openai_chat_to_anthropic; } response { sse_parse anthropic_to_openai_chunks; } } }`,
		`provider "unterminated`,
		`"\\`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, content string) {
		// These parsing entry points accept untrusted provider configuration. Their
		// error result is intentionally ignored: malformed input must be rejected
		// safely, while valid input must remain parseable by all entry points.
		_, _ = findProviderName("fuzz.conf", content)
		_, _, _, _, _, _, _, _, _, _ = parseProviderConfig("fuzz.conf", content)
		_, _ = ListProviderBlocks("fuzz.conf", content)
	})
}

func FuzzProviderBlockOperations(f *testing.F) {
	for _, seed := range []string{
		`provider "openai" { defaults { upstream_config { base_url = "https://api.openai.com"; } } }`,
		`provider "one" {} provider "two" { metadata { display_name = "Two"; } }`,
		`# a comment\nprovider "demo" { defaults { request { req_map openai_chat_to_anthropic; } } }`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, content string) {
		blocks, err := ListProviderBlocks("fuzz.conf", content)
		if err != nil {
			return
		}
		for _, block := range blocks {
			if block.Start < 0 || block.End < block.Start || block.End > len(content) {
				t.Fatalf("invalid block offsets: %#v", block)
			}
			if got := content[block.Start:block.End]; got != block.Content {
				t.Fatalf("block content does not match source: got %q want %q", block.Content, got)
			}
			if strings.TrimSpace(block.Name) == "" {
				continue
			}
			extracted, ok, err := ExtractProviderBlockOptional("fuzz.conf", content, block.Name)
			if err != nil || !ok || extracted != block.Content {
				t.Fatalf("ExtractProviderBlockOptional(%q) = (%q, %v, %v), want (%q, true, nil)", block.Name, extracted, ok, err, block.Content)
			}

			updated, err := UpsertProviderBlock("fuzz.conf", content, block.Name, block.Content)
			if err != nil {
				t.Fatalf("UpsertProviderBlock: %v", err)
			}
			got, ok, err := ExtractProviderBlockOptional("fuzz.conf", updated, block.Name)
			want := strings.TrimSpace(block.Content)
			if err != nil || !ok || got != want {
				t.Fatalf("upserted block = (%q, %v, %v), want (%q, true, nil)", got, ok, err, want)
			}
		}
	})
}
