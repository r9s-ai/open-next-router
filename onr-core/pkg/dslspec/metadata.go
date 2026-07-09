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
	Name              string
	Block             string
	Hover             string
	IsBlock           bool
	BlockHeader       bool
	Modes             []string
	ModeRegistryBlock string
	Args              []DirectiveArg
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
	{Name: "include", Block: "top", Hover: "`include path.conf;`\n\nIncludes another DSL fragment file before parsing. Supports unquoted nginx-style paths like `providers;` and `providers/*.conf;`."},
	{Name: "provider", Block: "top", Hover: "`provider \"name\" { ... }`\n\nDefines one provider DSL block. File name should match provider name.", IsBlock: true, BlockHeader: true},
	{Name: "usage_mode", Block: "top", Hover: "`usage_mode \"name\" { ... }`\n\nDefines one reusable global usage extraction preset.", IsBlock: true, BlockHeader: true},
	{Name: "finish_reason_mode", Block: "top", Hover: "`finish_reason_mode \"name\" { ... }`\n\nDefines one reusable global finish reason extraction preset.", IsBlock: true, BlockHeader: true},
	{Name: "models_mode", Block: "top", Hover: "`models_mode \"name\" { ... }`\n\nDefines one reusable global models query preset.", IsBlock: true, BlockHeader: true},
	{Name: "balance_mode", Block: "top", Hover: "`balance_mode \"name\" { ... }`\n\nDefines one reusable global balance query preset.", IsBlock: true, BlockHeader: true},

	{Name: "defaults", Block: "provider", Hover: "`defaults { ... }`\n\nDefault phases shared by all `match` rules unless overridden.", IsBlock: true},
	{Name: "match", Block: "provider", Hover: "`match api = \"...\" [stream = true|false] { ... }`\n\nRoute rule. First match wins.", IsBlock: true, BlockHeader: true},
	{Name: "metadata", Block: "provider", Hover: "`metadata { provider_family <family>; signal_profile <profile>; }`\n\nDeclares provider identity and capacity signal profile metadata.", IsBlock: true},

	{Name: "provider_family", Block: "metadata", Hover: "`provider_family <family>;`\n\nProvider family used for operations, debug output, and later capacity-signal grouping."},
	{Name: "signal_profile", Block: "metadata", Hover: "`signal_profile <profile>;`\n\nSignal profile used by later provider capacity signal adaptors."},

	{Name: "upstream_config", Block: "defaults", Hover: "`upstream_config { base_url = \"...\"; }`\n\nProvider-level upstream base URL config.", IsBlock: true},
	{Name: "auth", Block: "defaults", Hover: "`auth { ... }`\n\nAuthentication directives for upstream requests.", IsBlock: true},
	{Name: "request", Block: "defaults", Hover: "`request { ... }`\n\nRequest rewrite/transform directives.", IsBlock: true},
	{Name: "response", Block: "defaults", Hover: "`response { ... }`\n\nDownstream response mapping/transformation directives.", IsBlock: true},
	{Name: "error", Block: "defaults", Hover: "`error { error_map <mode>; }`\n\nNormalize upstream error payloads.", IsBlock: true},
	{Name: "metrics", Block: "defaults", Hover: "`metrics { ... }`\n\nToken usage and finish reason extraction rules.", IsBlock: true},
	{Name: "balance", Block: "defaults", Hover: "`balance { ... }`\n\nBalance query and extraction directives.", IsBlock: true},
	{Name: "models", Block: "defaults", Hover: "`models { ... }`\n\nProvider models list query and mapping directives.", IsBlock: true},

	{Name: "upstream", Block: "match", Hover: "`upstream { ... }`\n\nUpstream path/query routing directives.", IsBlock: true},
	{Name: "auth", Block: "match", Hover: "`auth { ... }`\n\nAuthentication directives for upstream requests.", IsBlock: true},
	{Name: "request", Block: "match", Hover: "`request { ... }`\n\nRequest rewrite/transform directives.", IsBlock: true},
	{Name: "response", Block: "match", Hover: "`response { ... }`\n\nDownstream response mapping/transformation directives.", IsBlock: true},
	{Name: "error", Block: "match", Hover: "`error { error_map <mode>; }`\n\nNormalize upstream error payloads.", IsBlock: true},
	{Name: "metrics", Block: "match", Hover: "`metrics { ... }`\n\nToken usage and finish reason extraction rules.", IsBlock: true},

	{Name: "usage_extract", Block: "usage_mode", Hover: "`usage_extract <mode>;`\n\nSelects `custom` or inherits another reusable `usage_mode` preset.", Modes: []string{"custom"}, ModeRegistryBlock: "usage_mode"},
	{Name: "usage_root", Block: "usage_mode", Hover: "`usage_root path=\"$.usage\" [event=\"a|b\"] [event_optional=true] [exclude=\"field_a|field_b\"];`\n\nExtracts and merges the upstream usage JSON object before `usage_fact` rules run. When a mode has `usage_root`, `usage_fact` without `source` reads from that merged usage object. `exclude` removes top-level keys from the extracted usage object before merging."},
	{Name: "usage_fact", Block: "usage_mode", Hover: "`usage_fact <dimension> <unit> path=\"$.path\"|count_path=\"$.path\"|sum_path=\"$.path\"|len_path=\"$.path\"|expr=\"<expr>\" ...;`\n\nAdds one usage fact extraction rule to a reusable `usage_mode` preset.\n\nCurrent `source` values: `usage`, `response`, `request`, `derived`. Empty `source` reads from `usage_root` when configured, otherwise from `response`.\n`len_path` yields the rune count of the string at the path (character billing). `when_path=\"$.x\" when_eq=\"v\"` gates the fact: it only matches when the value at `when_path` equals `when_eq`; missing paths never match.\nRestricted filter JSONPath is supported, for example `$.usageMetadata.promptTokensDetails[?(@.modality==\\\"AUDIO\\\")].tokenCount`."},
	{Name: "input_tokens_expr", Block: "usage_mode", Hover: "`input_tokens_expr = <expr>;`\n\nCustom extraction expression for input/prompt tokens in a reusable `usage_mode` preset."},
	{Name: "output_tokens_expr", Block: "usage_mode", Hover: "`output_tokens_expr = <expr>;`\n\nCustom extraction expression for output/completion tokens in a reusable `usage_mode` preset."},
	{Name: "cache_read_tokens_expr", Block: "usage_mode", Hover: "`cache_read_tokens_expr = <expr>;`\n\nCustom extraction expression for cache read tokens in a reusable `usage_mode` preset."},
	{Name: "cache_write_tokens_expr", Block: "usage_mode", Hover: "`cache_write_tokens_expr = <expr>;`\n\nCustom extraction expression for cache write tokens in a reusable `usage_mode` preset."},
	{Name: "total_tokens_expr", Block: "usage_mode", Hover: "`total_tokens_expr = <expr>;`\n\nCustom extraction expression for total tokens in a reusable `usage_mode` preset."},
	{Name: "input_tokens_path", Block: "usage_mode", Hover: "`input_tokens_path \"$.path\";`\n\nPath override for input token extraction in a reusable `usage_mode` preset."},
	{Name: "output_tokens_path", Block: "usage_mode", Hover: "`output_tokens_path \"$.path\";`\n\nPath override for output token extraction in a reusable `usage_mode` preset."},
	{Name: "cache_read_tokens_path", Block: "usage_mode", Hover: "`cache_read_tokens_path \"$.path\";`\n\nPath override for cache-read token extraction in a reusable `usage_mode` preset."},
	{Name: "cache_write_tokens_path", Block: "usage_mode", Hover: "`cache_write_tokens_path \"$.path\";`\n\nPath override for cache-write token extraction in a reusable `usage_mode` preset."},

	{Name: "finish_reason_extract", Block: "finish_reason_mode", Hover: "`finish_reason_extract <mode>;`\n\nSelects `custom` or inherits another reusable `finish_reason_mode` preset.", Modes: []string{"custom"}, ModeRegistryBlock: "finish_reason_mode"},
	{Name: "finish_reason_path", Block: "finish_reason_mode", Hover: "`finish_reason_path \"$.path\";`\n\nPath override for finish_reason extraction in a reusable `finish_reason_mode` preset."},

	{Name: "models_mode", Block: "models_mode", Hover: "`models_mode <mode>;`\n\nSelects `openai`, `gemini`, `custom`, or another reusable `models_mode` preset.", Modes: []string{"openai", "gemini", "custom"}, ModeRegistryBlock: "models_mode"},
	{Name: "method", Block: "models_mode", Hover: "`method GET|POST;`\n\nHTTP method used by models query endpoint.", Args: []DirectiveArg{{Name: "method", Kind: "enum", Enum: []string{"GET", "POST"}}}},
	{Name: "path", Block: "models_mode", Hover: "`path <expr>;`\n\nPath for models query endpoint."},
	{Name: "id_path", Block: "models_mode", Hover: "`id_path \"$.path\";`\n\nJSON path to extract model id(s) from models response."},
	{Name: "id_regex", Block: "models_mode", Hover: "`id_regex \"<regex>\";`\n\nRegex rewrite applied to extracted model ids."},
	{Name: "id_allow_regex", Block: "models_mode", Hover: "`id_allow_regex \"<regex>\";`\n\nFilter extracted model ids by regex allowlist."},
	{Name: "set_header", Block: "models_mode", Hover: "`set_header <Header-Name> <expr>;`\n\nSets header for models query request."},
	{Name: "del_header", Block: "models_mode", Hover: "`del_header <Header-Name>;`\n\nDeletes header for models query request."},

	{Name: "balance_mode", Block: "balance_mode", Hover: "`balance_mode <mode>;`\n\nSelects `openai`, `custom`, or another reusable `balance_mode` preset.", Modes: []string{"openai", "custom"}, ModeRegistryBlock: "balance_mode"},
	{Name: "method", Block: "balance_mode", Hover: "`method GET|POST;`\n\nHTTP method used by balance query endpoint.", Args: []DirectiveArg{{Name: "method", Kind: "enum", Enum: []string{"GET", "POST"}}}},
	{Name: "path", Block: "balance_mode", Hover: "`path <expr>;`\n\nPath for balance query endpoint (required in custom mode)."},
	{Name: "balance_path", Block: "balance_mode", Hover: "`balance_path \"$.path\";`\n\nJSON path used to read balance amount from response."},
	{Name: "used_path", Block: "balance_mode", Hover: "`used_path \"$.path\";`\n\nJSON path used to read used amount from response."},
	{Name: "balance_unit", Block: "balance_mode", Hover: "`balance_unit <unit>;`\n\nBalance currency/unit label (e.g. USD).", Args: []DirectiveArg{{Name: "unit", Kind: "enum", Enum: []string{"USD", "CNY"}}}},
	{Name: "subscription_path", Block: "balance_mode", Hover: "`subscription_path <path>;`\n\nOptional path to query subscription endpoint."},
	{Name: "usage_path", Block: "balance_mode", Hover: "`usage_path <path>;`\n\nOptional path to query usage endpoint."},
	{Name: "balance_expr", Block: "balance_mode", Hover: "`balance_expr = <expr>;`\n\nCustom expression for balance value extraction."},
	{Name: "used_expr", Block: "balance_mode", Hover: "`used_expr = <expr>;`\n\nCustom expression for used value extraction."},
	{Name: "set_header", Block: "balance_mode", Hover: "`set_header <Header-Name> <expr>;`\n\nSets header for balance query request."},
	{Name: "del_header", Block: "balance_mode", Hover: "`del_header <Header-Name>;`\n\nDeletes header for balance query request."},

	{Name: "base_url", Block: "upstream_config", Hover: "`base_url = \"https://...\";`\n\nSets provider default upstream base URL."},
	{Name: "transport", Block: "upstream_config", Hover: "`transport http|aws_sdk;`\n\nSelects upstream transport.", Args: []DirectiveArg{{Name: "transport", Kind: "enum", Enum: []string{"http", "aws_sdk"}}}},
	{Name: "set_path", Block: "upstream", Hover: "`set_path <expr>;`\n\nSets upstream request path."},
	{Name: "set_query", Block: "upstream", Hover: "`set_query <name> <expr>;`\n\nSets/upserts upstream query parameter."},
	{Name: "del_query", Block: "upstream", Hover: "`del_query <name>;`\n\nDeletes upstream query parameter."},

	{Name: "auth_bearer", Block: "auth", Hover: "`auth_bearer;`\n\nSets `Authorization: Bearer <channel.key>`."},
	{Name: "auth_header_key", Block: "auth", Hover: "`auth_header_key <Header-Name>;`\n\nSets `<Header-Name>: <channel.key>`."},
	{Name: "auth_oauth_bearer", Block: "auth", Hover: "`auth_oauth_bearer;`\n\nSets `Authorization: Bearer <oauth.access_token>`."},
	{Name: "auth_sigv4_bedrock", Block: "auth", Hover: "`auth_sigv4_bedrock;`\n\nDeclares AWS Bedrock SigV4 credentials for AWS SDK transport."},
	{Name: "oauth_mode", Block: "auth", Hover: "`oauth_mode <mode>;`\n\nEnable OAuth token fetch mode for upstream auth.", Modes: []string{"openai", "gemini", "qwen", "claude", "iflow", "antigravity", "kimi", "google_service_account_file", "custom"}},
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
	{Name: "json_replace", Block: "request", Hover: "`json_replace <jsonpath> <expr>;`\n\nReplaces one request JSON field only when the path already exists."},
	{Name: "json_set_if_absent", Block: "request", Hover: "`json_set_if_absent <jsonpath> <expr>;`\n\nSets JSON field only when target field is absent."},
	{Name: "json_del", Block: "request", Hover: "`json_del <jsonpath>;`\n\nDeletes one request JSON field."},
	{Name: "json_rename", Block: "request", Hover: "`json_rename <from-jsonpath> <to-jsonpath>;`\n\nRenames/moves one request JSON field."},
	{Name: "json_wrap_input_text", Block: "request", Hover: "`json_wrap_input_text <jsonpath>;`\n\nWraps a string field as an OpenAI Responses `input` message list. Missing paths and already-array values are left unchanged."},
	{Name: "json_set_header_values", Block: "request", Hover: "`json_set_header_values <jsonpath> <Header-Name> [separator=\"<sep>\"];`\n\nSets one request JSON array field from downstream header values."},
	{Name: "json_filter_values", Block: "request", Hover: "`json_filter_values <jsonpath> <pattern>...;`\n\nFilters one request JSON string array field by allowed values."},
	{Name: "json_del_with_condition", Block: "request", Hover: "`json_del_with_condition <jsonpath> <field> <pattern>...;`\n\nDeletes an object, or matching objects from an array, when the object's field matches one of the patterns."},
	{Name: "json_del_if_missing", Block: "request", Hover: "`json_del_if_missing <target-jsonpath> <required-jsonpath>;`\n\nDeletes the target request JSON field when the required JSON path is missing."},
	{Name: "json_map_value", Block: "request", Hover: "`json_map_value <jsonpath> \"<from>\" <to-expr>;` or block form `json_map_value <jsonpath> { \"<from>\" <to-expr>; ... }`\n\nReplaces the string value at path with the mapped result when it equals `<from>`. Unmatched values pass through unchanged (same fallthrough semantics as `model_map`). Use the block form to list many mappings for one path in a single directive."},
	{Name: "json_clamp", Block: "request", Hover: "`json_clamp <jsonpath> min=<f> max=<f>;`\n\nClamps the numeric value at path to `[min, max]`; values inside the range pass through unchanged. Missing/non-numeric fields are left unchanged."},
	{Name: "after_req_map", Block: "request", Hover: "`after_req_map { ... }`\n\nRuns nested request JSON operations after req_map. If no req_map is configured, runs after normal request JSON operations.", IsBlock: true},
	{Name: "req_map", Block: "request", Hover: "`req_map <mode>;`\n\nMap request JSON between API schemas.", Modes: []string{"openai_chat_to_openai_responses", "openai_chat_to_anthropic_messages", "openai_chat_to_gemini_generate_content", "anthropic_to_openai_chat", "gemini_to_openai_chat"}},
	{Name: "req_required", Block: "request", Hover: "`req_required <body|header|query> <path-or-name> [allow_null=true|false];`\n\nRejects the request with HTTP 400 when the target is missing. JSON null counts as missing unless allow_null=true (body source only). Runs after model_map and before request JSON operations."},
	{Name: "req_forbid", Block: "request", Hover: "`req_forbid <body|header|query> <path-or-name>;`\n\nRejects the request with HTTP 400 when the target is present."},
	{Name: "req_type", Block: "request", Hover: "`req_type body <jsonpath> <null|bool|number|integer|string|array|object>;`\n\nRejects the request with HTTP 400 when the body field exists but is not of the given JSON type. Missing fields pass; body source only."},
	{Name: "req_range", Block: "request", Hover: "`req_range <body|header|query> <path-or-name> [min=<number>] [max=<number>];`\n\nRejects the request with HTTP 400 when the target number is out of range. Missing fields pass. At least one of min/max is required."},
	{Name: "req_len", Block: "request", Hover: "`req_len <body|header|query> <path-or-name> [min=<int>] [max=<int>];`\n\nRejects the request with HTTP 400 when the string length (Unicode code points) or array length is out of range. Missing fields pass. At least one of min/max is required."},
	{Name: "req_enum", Block: "request", Hover: "`req_enum <body|header|query> <path-or-name> <value>...;`\n\nRejects the request with HTTP 400 when the target value is not one of the listed candidates. Body candidates are JSON literals (string, number, true, false, null); header/query candidates compare as strings. Missing fields pass."},

	{Name: "json_set", Block: "after_req_map", Hover: "`json_set <jsonpath> <expr>;`\n\nSets one request JSON field value after req_map."},
	{Name: "json_replace", Block: "after_req_map", Hover: "`json_replace <jsonpath> <expr>;`\n\nReplaces one request JSON field after req_map only when the path already exists."},
	{Name: "json_set_if_absent", Block: "after_req_map", Hover: "`json_set_if_absent <jsonpath> <expr>;`\n\nSets one request JSON field after req_map only when target field is absent."},
	{Name: "json_del", Block: "after_req_map", Hover: "`json_del <jsonpath>;`\n\nDeletes one request JSON field after req_map."},
	{Name: "json_rename", Block: "after_req_map", Hover: "`json_rename <from-jsonpath> <to-jsonpath>;`\n\nRenames/moves one request JSON field after req_map."},
	{Name: "json_wrap_input_text", Block: "after_req_map", Hover: "`json_wrap_input_text <jsonpath>;`\n\nWraps a string field as an OpenAI Responses `input` message list after req_map."},
	{Name: "json_set_header_values", Block: "after_req_map", Hover: "`json_set_header_values <jsonpath> <Header-Name> [separator=\"<sep>\"];`\n\nSets one request JSON array field from downstream header values after req_map."},
	{Name: "json_filter_values", Block: "after_req_map", Hover: "`json_filter_values <jsonpath> <pattern>...;`\n\nFilters one request JSON string array field by allowed values after req_map."},
	{Name: "json_del_with_condition", Block: "after_req_map", Hover: "`json_del_with_condition <jsonpath> <field> <pattern>...;`\n\nDeletes matching request JSON objects after req_map when the object's field matches one of the patterns."},
	{Name: "json_del_if_missing", Block: "after_req_map", Hover: "`json_del_if_missing <target-jsonpath> <required-jsonpath>;`\n\nDeletes the target request JSON field after req_map when the required JSON path is missing."},

	{Name: "resp_passthrough", Block: "response", Hover: "`resp_passthrough;`\n\nPasses upstream response through without schema mapping."},
	{Name: "resp_map", Block: "response", Hover: "`resp_map <mode>;`\n\nMap non-stream response JSON.", Modes: []string{"openai_responses_to_openai_chat", "anthropic_to_openai_chat", "gemini_to_openai_chat", "openai_to_anthropic_messages", "openai_to_gemini_chat", "openai_to_gemini_generate_content"}},
	{Name: "sse_parse", Block: "response", Hover: "`sse_parse <mode>;`\n\nMap streaming SSE events/chunks.", Modes: []string{"openai_responses_to_openai_chat_chunks", "anthropic_to_openai_chunks", "openai_to_anthropic_chunks", "openai_to_gemini_chunks", "gemini_to_openai_chat_chunks"}},
	{Name: "sse_collect", Block: "response", Hover: "`sse_collect <mode>;`\n\nCollects upstream SSE into the same protocol's non-stream JSON before optional `resp_map`/JSON ops.", Modes: []string{"openai_responses", "anthropic_messages", "gemini_generate_content"}},
	{Name: "json_set", Block: "response", Hover: "`json_set <jsonpath> <expr> [event=\"a|b\"] [event_optional=true] [max_count=n];`\n\nSets one downstream response JSON field value (best-effort)."},
	{Name: "json_replace", Block: "response", Hover: "`json_replace <jsonpath> <expr> [event=\"a|b\"] [event_optional=true] [max_count=n];`\n\nReplaces one downstream response JSON field only when the path already exists."},
	{Name: "json_set_if_absent", Block: "response", Hover: "`json_set_if_absent <jsonpath> <expr> [event=\"a|b\"] [event_optional=true] [max_count=n];`\n\nSets response JSON field only when absent (best-effort)."},
	{Name: "json_del", Block: "response", Hover: "`json_del <jsonpath> [event=\"a|b\"] [event_optional=true] [max_count=n];`\n\nDeletes one downstream response JSON field (best-effort)."},
	{Name: "json_rename", Block: "response", Hover: "`json_rename <from-jsonpath> <to-jsonpath> [event=\"a|b\"] [event_optional=true] [max_count=n];`\n\nRenames/moves one downstream response JSON field (best-effort)."},
	{Name: "sse_json_del_if", Block: "response", Hover: "`sse_json_del_if <cond-jsonpath> <equals-string> <del-jsonpath>;`\n\nFor SSE JSON event payloads, conditionally delete one field."},
	{Name: "resp_body_extract", Block: "response", Hover: "`resp_body_extract path=\"$.data.audio\" decode=hex|base64;`\n\nNon-stream: decodes the string at path from the upstream JSON response into the raw downstream body (e.g. hex-encoded audio -> binary audio). Usage/error extraction reads the original JSON before the body transform."},
	{Name: "resp_content_type", Block: "response", Hover: "`resp_content_type from_path=\"$.path\" kind=audio [default=\"mp3\"];`\n\nNon-stream: resolves the downstream Content-Type from a format field in the upstream JSON response. Used together with `resp_body_extract`."},
	{Name: "sse_binary_extract", Block: "response", Hover: "`sse_binary_extract path=\"$.data.audio\" decode=hex|base64 [stop_path=\"$.data.status\" stop_eq=<value>];`\n\nStream: decodes the string at path from each upstream SSE JSON chunk and writes raw bytes to the downstream body (SSE -> binary stream). When the value at stop_path equals stop_eq, the stream ends."},

	{Name: "error_map", Block: "error", Hover: "`error_map <mode>;`\n\nNormalize upstream error payload into target error schema.", Modes: []string{"openai", "common", "passthrough"}},
	{Name: "error_when", Block: "error", Hover: "`error_when path=\"$.base_resp.status_code\" ne=<value>|eq=<value> [status=400];`\n\nDetects in-body upstream errors on HTTP 2xx responses. Matching responses are treated as upstream errors and normalized via `error_map`. Missing paths never match."},

	{Name: "usage_extract", Block: "metrics", Hover: "`usage_extract <mode>;`\n\nExtract usage token fields from response/SSE payload. Supports `custom` and user-defined global `usage_mode` presets.", Modes: []string{"custom"}, ModeRegistryBlock: "usage_mode"},
	{Name: "usage_root", Block: "metrics", Hover: "`usage_root path=\"$.usage\" [event=\"a|b\"] [event_optional=true] [exclude=\"field_a|field_b\"];`\n\nExtracts and merges the upstream usage JSON object before `usage_fact` rules run. When a metrics block has `usage_root`, `usage_fact` without `source` reads from that merged usage object. `exclude` removes top-level keys from the extracted usage object before merging."},
	{Name: "usage_fact", Block: "metrics", Hover: "`usage_fact <dimension> <unit> path=\"$.path\"|count_path=\"$.path\"|sum_path=\"$.path\"|len_path=\"$.path\"|expr=\"<expr>\" ...;`\n\nCustom usage fact extraction rule with optional `attr.*` and `fallback=true`.\n\nCurrent `source` values: `usage`, `response`, `request`, `derived`. Empty `source` reads from `usage_root` when configured, otherwise from `response`.\n`len_path` yields the rune count of the string at the path (character billing). `when_path=\"$.x\" when_eq=\"v\"` gates the fact: it only matches when the value at `when_path` equals `when_eq`; missing paths never match.\nRestricted filter JSONPath is supported, for example `$.usageMetadata.promptTokensDetails[?(@.modality==\\\"AUDIO\\\")].tokenCount`."},
	{Name: "input_tokens_expr", Block: "metrics", Hover: "`input_tokens_expr = <expr>;`\n\nCustom extraction expression for input/prompt tokens."},
	{Name: "output_tokens_expr", Block: "metrics", Hover: "`output_tokens_expr = <expr>;`\n\nCustom extraction expression for output/completion tokens."},
	{Name: "cache_read_tokens_expr", Block: "metrics", Hover: "`cache_read_tokens_expr = <expr>;`\n\nCustom extraction expression for cache read tokens."},
	{Name: "cache_write_tokens_expr", Block: "metrics", Hover: "`cache_write_tokens_expr = <expr>;`\n\nCustom extraction expression for cache write tokens."},
	{Name: "total_tokens_expr", Block: "metrics", Hover: "`total_tokens_expr = <expr>;`\n\nCustom extraction expression for total tokens."},
	{Name: "input_tokens_path", Block: "metrics", Hover: "`input_tokens_path \"$.path\";`\n\nPath override for input token extraction (custom mode)."},
	{Name: "output_tokens_path", Block: "metrics", Hover: "`output_tokens_path \"$.path\";`\n\nPath override for output token extraction (custom mode)."},
	{Name: "cache_read_tokens_path", Block: "metrics", Hover: "`cache_read_tokens_path \"$.path\";`\n\nPath override for cache-read token extraction (custom mode)."},
	{Name: "cache_write_tokens_path", Block: "metrics", Hover: "`cache_write_tokens_path \"$.path\";`\n\nPath override for cache-write token extraction (custom mode)."},
	{Name: "finish_reason_extract", Block: "metrics", Hover: "`finish_reason_extract <mode>;`\n\nExtract finish_reason from response/SSE payload. Supports `custom` and user-defined global `finish_reason_mode` presets.", Modes: []string{"custom"}, ModeRegistryBlock: "finish_reason_mode"},
	{Name: "finish_reason_path", Block: "metrics", Hover: "`finish_reason_path \"$.path\";`\n\nPath override for finish_reason extraction (custom mode)."},

	{Name: "balance_mode", Block: "balance", Hover: "`balance_mode <mode>;`\n\nSelects `openai`, `custom`, or a user-defined global `balance_mode` preset.", Modes: []string{"openai", "custom"}, ModeRegistryBlock: "balance_mode"},
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

	{Name: "models_mode", Block: "models", Hover: "`models_mode <mode>;`\n\nSelects `openai`, `gemini`, `custom`, or a user-defined global `models_mode` preset.", Modes: []string{"openai", "gemini", "custom"}, ModeRegistryBlock: "models_mode"},
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

// ModesByDirectiveInBlock returns allowed mode values for one directive in a
// specific parent block.
func ModesByDirectiveInBlock(name, block string) []string {
	key := strings.TrimSpace(name)
	if key == "" {
		return nil
	}
	b := normalizeMetaBlock(block)
	for _, d := range directiveMetadata {
		if d.Name != key || normalizeMetaBlock(d.Block) != b {
			continue
		}
		return append([]string(nil), d.Modes...)
	}
	return nil
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

// ModeDirectiveNamesInBlock returns directive names that accept built-in mode
// values in a specific parent block.
func ModeDirectiveNamesInBlock(block string) []string {
	b := normalizeMetaBlock(block)
	if b == "" {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	for _, d := range directiveMetadata {
		if normalizeMetaBlock(d.Block) != b || len(d.Modes) == 0 {
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

// DirectiveModeRegistryBlock returns the top-level preset block name used to
// resolve user-defined mode values for a directive in one block.
func DirectiveModeRegistryBlock(name, block string) string {
	key := strings.TrimSpace(name)
	if key == "" {
		return ""
	}
	b := normalizeMetaBlock(block)
	if out := directiveModeRegistryBlock(key, b, true); out != "" {
		return out
	}
	return directiveModeRegistryBlock(key, b, false)
}

// DirectiveModeRegistryBlockInBlock returns the top-level preset block name
// used to resolve user-defined mode values for a directive in a specific parent
// block.
func DirectiveModeRegistryBlockInBlock(name, block string) string {
	key := strings.TrimSpace(name)
	if key == "" {
		return ""
	}
	return directiveModeRegistryBlock(key, normalizeMetaBlock(block), true)
}

// DirectiveHasDynamicModeRegistry reports whether this directive can resolve
// additional mode names from top-level reusable preset blocks.
func DirectiveHasDynamicModeRegistry(name string) bool {
	key := strings.TrimSpace(name)
	if key == "" {
		return false
	}
	for _, d := range directiveMetadata {
		if d.Name != key || strings.TrimSpace(d.ModeRegistryBlock) == "" {
			continue
		}
		return true
	}
	return false
}

// DirectiveHasDynamicModeRegistryInBlock reports whether this directive can
// resolve additional mode names in a specific parent block.
func DirectiveHasDynamicModeRegistryInBlock(name, block string) bool {
	return DirectiveModeRegistryBlockInBlock(name, block) != ""
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

// DirectiveIsBlockInBlock reports whether name opens a nested block in parent block.
func DirectiveIsBlockInBlock(name, block string) bool {
	key := strings.TrimSpace(name)
	if key == "" {
		return false
	}
	b := normalizeMetaBlock(block)
	for _, d := range directiveMetadata {
		if d.Name != key || normalizeMetaBlock(d.Block) != b {
			continue
		}
		return d.IsBlock
	}
	return false
}

// DirectiveBlockHasHeaderInBlock reports whether a block directive consumes
// header arguments before its opening brace, for example provider "name" { ... }.
func DirectiveBlockHasHeaderInBlock(name, block string) bool {
	key := strings.TrimSpace(name)
	if key == "" {
		return false
	}
	b := normalizeMetaBlock(block)
	for _, d := range directiveMetadata {
		if d.Name != key || normalizeMetaBlock(d.Block) != b {
			continue
		}
		return d.IsBlock && d.BlockHeader
	}
	return false
}

// BlockDirectiveNames returns block directive keywords.
func BlockDirectiveNames() []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 16)
	for _, d := range directiveMetadata {
		name := strings.TrimSpace(d.Name)
		if name == "" || !d.IsBlock {
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

// IsBlockDirective reports whether name is a block directive keyword.
func IsBlockDirective(name string) bool {
	key := strings.TrimSpace(name)
	if key == "" {
		return false
	}
	for _, d := range directiveMetadata {
		if d.Name == key && d.IsBlock {
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

func directiveModeRegistryBlock(name, block string, matchBlock bool) string {
	for _, d := range directiveMetadata {
		if d.Name != name {
			continue
		}
		if matchBlock && normalizeMetaBlock(d.Block) != block {
			continue
		}
		registryBlock := normalizeMetaBlock(d.ModeRegistryBlock)
		if registryBlock == "" {
			continue
		}
		return registryBlock
	}
	return ""
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
