package dslspec

import (
	"sort"
	"strings"
)

// DirectiveMetadata describes one DSL directive's editor-facing metadata.
// Block uses normalized names:
// - "top" for file-level statements.
// - other values match block names in DSL (provider/defaults/auth/request/...).
type DirectiveMetadata struct {
	Name  string
	Block string
	Hover string
	Modes []string
	Args  []DirectiveArg
}

// DirectiveArg describes one positional argument for a directive.
// Kind currently supports:
// - "enum": one value from Enum list.
type DirectiveArg struct {
	Name string
	Kind string
	Enum []string
}

var directiveMetadata = []DirectiveMetadata{
	{Name: "syntax", Block: "top", Hover: "`syntax \"next-router/0.1\";`\n\nDeclares DSL syntax version for this file."},
	{Name: "include", Block: "top", Hover: "`include \"path.conf\";`\n\nIncludes another DSL fragment file before parsing this provider file."},
	{Name: "provider", Block: "top", Hover: "`provider \"name\" { ... }`\n\nDefines one provider DSL block. File name should match provider name."},

	{Name: "defaults", Block: "provider", Hover: "`defaults { ... }`\n\nDefault phases shared by all `match` rules unless overridden."},
	{Name: "match", Block: "provider", Hover: "`match api = \"...\" [stream = true|false] { ... }`\n\nRoute rule. First match wins."},

	{Name: "upstream_config", Block: "defaults", Hover: "`upstream_config { base_url = \"...\"; }`\n\nProvider-level upstream base URL config."},
	{Name: "auth", Block: "defaults", Hover: "`auth { ... }`\n\nAuthentication directives for upstream requests."},
	{Name: "request", Block: "defaults", Hover: "`request { ... }`\n\nRequest rewrite/transform directives."},
	{Name: "response", Block: "defaults", Hover: "`response { ... }`\n\nDownstream response mapping/transformation directives."},
	{Name: "error", Block: "defaults", Hover: "`error { error_map <mode>; }`\n\nNormalize upstream error payloads."},
	{Name: "metrics", Block: "defaults", Hover: "`metrics { ... }`\n\nToken usage and finish reason extraction rules."},
	{Name: "balance", Block: "defaults", Hover: "`balance { ... }`\n\nBalance query and extraction directives."},
	{Name: "models", Block: "defaults", Hover: "`models { ... }`\n\nProvider models list query and mapping directives."},

	{Name: "upstream", Block: "match", Hover: "`upstream { ... }`\n\nUpstream path/query routing directives."},
	{Name: "auth", Block: "match", Hover: "`auth { ... }`\n\nAuthentication directives for upstream requests."},
	{Name: "request", Block: "match", Hover: "`request { ... }`\n\nRequest rewrite/transform directives."},
	{Name: "response", Block: "match", Hover: "`response { ... }`\n\nDownstream response mapping/transformation directives."},
	{Name: "error", Block: "match", Hover: "`error { error_map <mode>; }`\n\nNormalize upstream error payloads."},
	{Name: "metrics", Block: "match", Hover: "`metrics { ... }`\n\nToken usage and finish reason extraction rules."},

	{Name: "base_url", Block: "upstream_config", Hover: "`base_url = \"https://...\";`\n\nSets provider default upstream base URL."},
	{Name: "set_path", Block: "upstream", Hover: "`set_path <expr>;`\n\nSets upstream request path."},
	{Name: "set_query", Block: "upstream", Hover: "`set_query <name> <expr>;`\n\nSets/upserts upstream query parameter."},
	{Name: "del_query", Block: "upstream", Hover: "`del_query <name>;`\n\nDeletes upstream query parameter."},

	{Name: "auth_bearer", Block: "auth", Hover: "`auth_bearer;`\n\nSets `Authorization: Bearer <channel.key>`."},
	{Name: "auth_header_key", Block: "auth", Hover: "`auth_header_key <Header-Name>;`\n\nSets `<Header-Name>: <channel.key>`."},
	{Name: "auth_oauth_bearer", Block: "auth", Hover: "`auth_oauth_bearer;`\n\nSets `Authorization: Bearer <oauth.access_token>`."},
	{Name: "oauth_mode", Block: "auth", Hover: "`oauth_mode <mode>;`\n\nEnable OAuth token fetch mode for upstream auth.", Modes: []string{"openai", "gemini", "qwen", "claude", "iflow", "antigravity", "kimi", "custom"}},
	{Name: "oauth_token_url", Block: "auth", Hover: "`oauth_token_url <expr>;`\n\nOverrides token endpoint URL (typically with `oauth_mode custom`)."},
	{Name: "oauth_client_id", Block: "auth", Hover: "`oauth_client_id <expr>;`\n\nSets OAuth client id expression for token exchange."},
	{Name: "oauth_client_secret", Block: "auth", Hover: "`oauth_client_secret <expr>;`\n\nSets OAuth client secret expression for token exchange."},
	{Name: "oauth_refresh_token", Block: "auth", Hover: "`oauth_refresh_token <expr>;`\n\nSets OAuth refresh token expression for token exchange."},
	{Name: "oauth_scope", Block: "auth", Hover: "`oauth_scope <expr>;`\n\nSets OAuth scope expression for token exchange."},
	{Name: "oauth_audience", Block: "auth", Hover: "`oauth_audience <expr>;`\n\nSets OAuth audience expression for token exchange."},
	{Name: "oauth_method", Block: "auth", Hover: "`oauth_method GET|POST;`\n\nSets HTTP method for OAuth token request.", Args: []DirectiveArg{{Name: "method", Kind: "enum", Enum: []string{"GET", "POST"}}}},
	{Name: "oauth_content_type", Block: "auth", Hover: "`oauth_content_type form|json;`\n\nSets payload encoding for OAuth token request.", Args: []DirectiveArg{{Name: "content_type", Kind: "enum", Enum: []string{"form", "json"}}}},
	{Name: "oauth_token_path", Block: "auth", Hover: "`oauth_token_path \"$.path\";`\n\nJSONPath to extract access token from OAuth response."},
	{Name: "oauth_expires_in_path", Block: "auth", Hover: "`oauth_expires_in_path \"$.path\";`\n\nJSONPath to extract `expires_in` from OAuth response."},
	{Name: "oauth_token_type_path", Block: "auth", Hover: "`oauth_token_type_path \"$.path\";`\n\nJSONPath to extract token type from OAuth response."},
	{Name: "oauth_timeout_ms", Block: "auth", Hover: "`oauth_timeout_ms <int>;`\n\nSets timeout in milliseconds for OAuth token request."},
	{Name: "oauth_refresh_skew_sec", Block: "auth", Hover: "`oauth_refresh_skew_sec <int>;`\n\nRefresh token ahead of expiry by this many seconds."},
	{Name: "oauth_fallback_ttl_sec", Block: "auth", Hover: "`oauth_fallback_ttl_sec <int>;`\n\nFallback token TTL when provider does not return expires_in."},
	{Name: "oauth_form", Block: "auth", Hover: "`oauth_form <key> <expr>;`\n\nAdds one form field to OAuth token request body."},

	{Name: "set_header", Block: "request", Hover: "`set_header <Header-Name> <expr>;`\n\nSets or overrides one upstream request header."},
	{Name: "pass_header", Block: "request", Hover: "`pass_header <Header-Name>;`\n\nCopies one header from the original client request to the upstream request."},
	{Name: "filter_header_values", Block: "request", Hover: "`filter_header_values <header> <pattern>... [separator=\"<sep>\"];`\n\nFilters itemized upstream request header values and removes matching entries."},
	{Name: "del_header", Block: "request", Hover: "`del_header <Header-Name>;`\n\nDeletes one upstream request header."},
	{Name: "model_map", Block: "request", Hover: "`model_map <from> <expr>;`\n\nMaps input model name to upstream model expression."},
	{Name: "model_map_default", Block: "request", Hover: "`model_map_default <expr>;`\n\nFallback mapped model expression when no rule matches."},
	{Name: "json_set", Block: "request", Hover: "`json_set <jsonpath> <expr>;`\n\nSets one request JSON field value."},
	{Name: "json_set_if_absent", Block: "request", Hover: "`json_set_if_absent <jsonpath> <expr>;`\n\nSets JSON field only when target field is absent."},
	{Name: "json_del", Block: "request", Hover: "`json_del <jsonpath>;`\n\nDeletes one request JSON field."},
	{Name: "json_rename", Block: "request", Hover: "`json_rename <from-jsonpath> <to-jsonpath>;`\n\nRenames/moves one request JSON field."},
	{Name: "req_map", Block: "request", Hover: "`req_map <mode>;`\n\nMap request JSON between API schemas.", Modes: []string{"openai_chat_to_openai_responses", "openai_chat_to_anthropic_messages", "openai_chat_to_gemini_generate_content", "anthropic_to_openai_chat", "gemini_to_openai_chat"}},

	{Name: "resp_passthrough", Block: "response", Hover: "`resp_passthrough;`\n\nPasses upstream response through without schema mapping."},
	{Name: "resp_map", Block: "response", Hover: "`resp_map <mode>;`\n\nMap non-stream response JSON.", Modes: []string{"openai_responses_to_openai_chat", "anthropic_to_openai_chat", "gemini_to_openai_chat", "openai_to_anthropic_messages", "openai_to_gemini_chat", "openai_to_gemini_generate_content"}},
	{Name: "sse_parse", Block: "response", Hover: "`sse_parse <mode>;`\n\nMap streaming SSE events/chunks.", Modes: []string{"openai_responses_to_openai_chat_chunks", "anthropic_to_openai_chunks", "openai_to_anthropic_chunks", "openai_to_gemini_chunks", "gemini_to_openai_chat_chunks"}},
	{Name: "json_set", Block: "response", Hover: "`json_set <jsonpath> <expr>;`\n\nSets one downstream response JSON field value (best-effort)."},
	{Name: "json_set_if_absent", Block: "response", Hover: "`json_set_if_absent <jsonpath> <expr>;`\n\nSets response JSON field only when absent (best-effort)."},
	{Name: "json_del", Block: "response", Hover: "`json_del <jsonpath>;`\n\nDeletes one downstream response JSON field (best-effort)."},
	{Name: "json_rename", Block: "response", Hover: "`json_rename <from-jsonpath> <to-jsonpath>;`\n\nRenames/moves one downstream response JSON field (best-effort)."},
	{Name: "sse_json_del_if", Block: "response", Hover: "`sse_json_del_if <cond-jsonpath> <equals-string> <del-jsonpath>;`\n\nFor SSE JSON event payloads, conditionally delete one field."},

	{Name: "error_map", Block: "error", Hover: "`error_map <mode>;`\n\nNormalize upstream error payload into target error schema.", Modes: []string{"openai", "common", "passthrough"}},

	{Name: "usage_extract", Block: "metrics", Hover: "`usage_extract <mode>;`\n\nExtract usage token fields from response/SSE payload.", Modes: []string{"openai", "anthropic", "gemini", "custom"}},
	{Name: "usage_fact", Block: "metrics", Hover: "`usage_fact <dimension> <unit> path=\"$.path\"|count_path=\"$.path\"|sum_path=\"$.path\"|expr=\"<expr>\" ...;`\n\nCustom usage fact extraction rule with optional `attr.*` and `fallback=true`.\n\nCurrent `source` values: `response`, `request`, `derived`.\nRestricted filter JSONPath is supported, for example `$.usageMetadata.promptTokensDetails[?(@.modality==\\\"AUDIO\\\")].tokenCount`."},
	{Name: "input_tokens_expr", Block: "metrics", Hover: "`input_tokens_expr = <expr>;`\n\nCustom extraction expression for input/prompt tokens."},
	{Name: "output_tokens_expr", Block: "metrics", Hover: "`output_tokens_expr = <expr>;`\n\nCustom extraction expression for output/completion tokens."},
	{Name: "cache_read_tokens_expr", Block: "metrics", Hover: "`cache_read_tokens_expr = <expr>;`\n\nCustom extraction expression for cache read tokens."},
	{Name: "cache_write_tokens_expr", Block: "metrics", Hover: "`cache_write_tokens_expr = <expr>;`\n\nCustom extraction expression for cache write tokens."},
	{Name: "total_tokens_expr", Block: "metrics", Hover: "`total_tokens_expr = <expr>;`\n\nCustom extraction expression for total tokens."},
	{Name: "input_tokens_path", Block: "metrics", Hover: "`input_tokens_path \"$.path\";`\n\nPath override for input token extraction (custom mode)."},
	{Name: "output_tokens_path", Block: "metrics", Hover: "`output_tokens_path \"$.path\";`\n\nPath override for output token extraction (custom mode)."},
	{Name: "cache_read_tokens_path", Block: "metrics", Hover: "`cache_read_tokens_path \"$.path\";`\n\nPath override for cache-read token extraction (custom mode)."},
	{Name: "cache_write_tokens_path", Block: "metrics", Hover: "`cache_write_tokens_path \"$.path\";`\n\nPath override for cache-write token extraction (custom mode)."},
	{Name: "finish_reason_extract", Block: "metrics", Hover: "`finish_reason_extract <mode>;`\n\nExtract finish_reason from response/SSE payload.", Modes: []string{"openai", "anthropic", "gemini", "custom"}},
	{Name: "finish_reason_path", Block: "metrics", Hover: "`finish_reason_path \"$.path\";`\n\nPath override for finish_reason extraction (custom mode)."},

	{Name: "balance_mode", Block: "balance", Hover: "`balance_mode <mode>;`\n\nSelects built-in or custom balance query mode.", Modes: []string{"openai", "custom"}},
	{Name: "method", Block: "balance", Hover: "`method GET|POST;`\n\nHTTP method used by balance query endpoint.", Args: []DirectiveArg{{Name: "method", Kind: "enum", Enum: []string{"GET", "POST"}}}},
	{Name: "path", Block: "balance", Hover: "`path <expr>;`\n\nPath for balance query endpoint (required in custom mode)."},
	{Name: "balance_path", Block: "balance", Hover: "`balance_path \"$.path\";`\n\nJSON path used to read balance amount from response."},
	{Name: "used_path", Block: "balance", Hover: "`used_path \"$.path\";`\n\nJSON path used to read used amount from response."},
	{Name: "balance_unit", Block: "balance", Hover: "`balance_unit <unit>;`\n\nBalance currency/unit label (e.g. USD).", Args: []DirectiveArg{{Name: "unit", Kind: "enum", Enum: []string{"USD", "CNY"}}}},
	{Name: "subscription_path", Block: "balance", Hover: "`subscription_path <path>;`\n\nOptional path to query subscription endpoint."},
	{Name: "usage_path", Block: "balance", Hover: "`usage_path <path>;`\n\nOptional path to query usage endpoint."},
	{Name: "balance_expr", Block: "balance", Hover: "`balance_expr = <expr>;`\n\nCustom expression for balance value extraction."},
	{Name: "used_expr", Block: "balance", Hover: "`used_expr = <expr>;`\n\nCustom expression for used value extraction."},
	{Name: "set_header", Block: "balance", Hover: "`set_header <Header-Name> <expr>;`\n\nSets header for balance query request."},
	{Name: "del_header", Block: "balance", Hover: "`del_header <Header-Name>;`\n\nDeletes header for balance query request."},

	{Name: "models_mode", Block: "models", Hover: "`models_mode <mode>;`\n\nSelects models list query mode.", Modes: []string{"openai", "gemini", "custom"}},
	{Name: "method", Block: "models", Hover: "`method GET|POST;`\n\nHTTP method used by models query endpoint.", Args: []DirectiveArg{{Name: "method", Kind: "enum", Enum: []string{"GET", "POST"}}}},
	{Name: "path", Block: "models", Hover: "`path <expr>;`\n\nPath for models query endpoint."},
	{Name: "id_path", Block: "models", Hover: "`id_path \"$.path\";`\n\nJSON path to extract model id(s) from models response."},
	{Name: "id_regex", Block: "models", Hover: "`id_regex \"<regex>\";`\n\nRegex rewrite applied to extracted model ids."},
	{Name: "id_allow_regex", Block: "models", Hover: "`id_allow_regex \"<regex>\";`\n\nFilter extracted model ids by regex allowlist."},
	{Name: "set_header", Block: "models", Hover: "`set_header <Header-Name> <expr>;`\n\nSets header for models query request."},
	{Name: "del_header", Block: "models", Hover: "`del_header <Header-Name>;`\n\nDeletes header for models query request."},
}

// DirectiveHover returns hover markdown for a directive name.
func DirectiveHover(name string) (string, bool) {
	key := strings.TrimSpace(name)
	if key == "" {
		return "", false
	}
	for _, d := range directiveMetadata {
		if d.Name != key || strings.TrimSpace(d.Hover) == "" {
			continue
		}
		return d.Hover, true
	}
	return "", false
}

// DirectiveHoverInBlock returns hover markdown for a directive in one block.
// It first tries exact block match, then falls back to global name-only match.
func DirectiveHoverInBlock(name, block string) (string, bool) {
	key := strings.TrimSpace(name)
	if key == "" {
		return "", false
	}
	b := normalizeMetaBlock(block)
	for _, d := range directiveMetadata {
		if d.Name != key || strings.TrimSpace(d.Hover) == "" {
			continue
		}
		if normalizeMetaBlock(d.Block) != b {
			continue
		}
		return d.Hover, true
	}
	return DirectiveHover(name)
}

// DirectivesByBlock returns directive names allowed in one block.
func DirectivesByBlock(block string) []string {
	b := normalizeMetaBlock(block)
	if b == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, d := range directiveMetadata {
		if normalizeMetaBlock(d.Block) != b {
			continue
		}
		if _, ok := seen[d.Name]; ok {
			continue
		}
		seen[d.Name] = struct{}{}
		out = append(out, d.Name)
	}
	return out
}

// ModesByDirective returns allowed mode values for one directive.
func ModesByDirective(name string) []string {
	key := strings.TrimSpace(name)
	if key == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, d := range directiveMetadata {
		if d.Name != key {
			continue
		}
		for _, m := range d.Modes {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}

// ModeDirectiveNames returns directive names that accept built-in mode values.
func ModeDirectiveNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, d := range directiveMetadata {
		if len(d.Modes) == 0 {
			continue
		}
		name := strings.TrimSpace(d.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// DirectiveAllowedBlocks returns block names where this directive is allowed.
// Returned block names are normalized ("top" for file-level).
func DirectiveAllowedBlocks(name string) []string {
	key := strings.TrimSpace(name)
	if key == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, d := range directiveMetadata {
		if d.Name != key {
			continue
		}
		block := normalizeMetaBlock(d.Block)
		if block == "" {
			continue
		}
		if _, ok := seen[block]; ok {
			continue
		}
		seen[block] = struct{}{}
		out = append(out, block)
	}
	sort.Strings(out)
	return out
}

// BlockDirectiveNames returns block directive keywords (excluding "top").
func BlockDirectiveNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, d := range directiveMetadata {
		block := normalizeMetaBlock(d.Block)
		if block == "" || block == "top" {
			continue
		}
		if _, ok := seen[block]; ok {
			continue
		}
		seen[block] = struct{}{}
		out = append(out, block)
	}
	sort.Strings(out)
	return out
}

// IsBlockDirective reports whether name is a block directive keyword.
func IsBlockDirective(name string) bool {
	key := strings.TrimSpace(name)
	if key == "" {
		return false
	}
	for _, block := range BlockDirectiveNames() {
		if block == key {
			return true
		}
	}
	return false
}

func normalizeMetaBlock(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "_top":
		return "top"
	default:
		return v
	}
}

// DirectiveArgEnumValuesInBlock returns enum values for one directive argument in one block.
// It first tries exact block match, then falls back to name-only lookup.
func DirectiveArgEnumValuesInBlock(name, block string, argIndex int) []string {
	if argIndex < 0 {
		return nil
	}
	key := strings.TrimSpace(name)
	if key == "" {
		return nil
	}
	b := normalizeMetaBlock(block)
	if out := directiveArgEnumValues(key, b, argIndex, true); len(out) > 0 {
		return out
	}
	return directiveArgEnumValues(key, b, argIndex, false)
}

func directiveArgEnumValues(name, block string, argIndex int, matchBlock bool) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, d := range directiveMetadata {
		if d.Name != name {
			continue
		}
		if matchBlock && normalizeMetaBlock(d.Block) != block {
			continue
		}
		if len(d.Args) <= argIndex {
			continue
		}
		arg := d.Args[argIndex]
		if strings.ToLower(strings.TrimSpace(arg.Kind)) != "enum" {
			continue
		}
		for _, v := range arg.Enum {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

// DirectiveMetadataList returns a copy of all directive metadata entries.
func DirectiveMetadataList() []DirectiveMetadata {
	out := make([]DirectiveMetadata, 0, len(directiveMetadata))
	for _, d := range directiveMetadata {
		copyItem := d
		if len(d.Modes) > 0 {
			copyItem.Modes = append([]string(nil), d.Modes...)
		}
		if len(d.Args) > 0 {
			argsCopy := make([]DirectiveArg, 0, len(d.Args))
			for _, a := range d.Args {
				argCopy := a
				if len(a.Enum) > 0 {
					argCopy.Enum = append([]string(nil), a.Enum...)
				}
				argsCopy = append(argsCopy, argCopy)
			}
			copyItem.Args = argsCopy
		}
		out = append(out, copyItem)
	}
	return out
}
