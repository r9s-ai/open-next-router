package dslconfig

import (
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestApplyJSONOps_ModelChannelMaxPriceIsJSONNumber(t *testing.T) {
	price := 2.5
	meta := &dslmeta.Meta{
		ModelChannelMaxPrice: &dslmeta.ModelChannelMaxPrice{Prompt: &price},
	}
	root, err := ApplyJSONOps(meta, map[string]any{}, []JSONOp{{
		Op: "json_set", Path: "$.provider.max_price.prompt", ValueExpr: "$model_channel.max_price.prompt",
	}})
	if err != nil {
		t.Fatalf("ApplyJSONOps: %v", err)
	}
	got := root["provider"].(map[string]any)["max_price"].(map[string]any)["prompt"]
	if got != price {
		t.Fatalf("prompt=%v (%T), want %v (float64)", got, got, price)
	}
}

func TestApplyJSONOps_ModelChannelMaxPriceExplicitZeroIsJSONNumber(t *testing.T) {
	price := 0.0
	meta := &dslmeta.Meta{
		ModelChannelMaxPrice: &dslmeta.ModelChannelMaxPrice{Prompt: &price},
	}
	root, err := ApplyJSONOps(meta, map[string]any{}, []JSONOp{{
		Op: "json_set", Path: "$.provider.max_price.prompt", ValueExpr: "$model_channel.max_price.prompt",
	}})
	if err != nil {
		t.Fatalf("ApplyJSONOps: %v", err)
	}
	got := root["provider"].(map[string]any)["max_price"].(map[string]any)["prompt"]
	if got != price {
		t.Fatalf("prompt=%v (%T), want explicit zero float64", got, got)
	}
}

func TestApplyJSONOps_ModelChannelMaxPriceMissingReturnsError(t *testing.T) {
	_, err := ApplyJSONOps(&dslmeta.Meta{
		ModelChannelMaxPrice: &dslmeta.ModelChannelMaxPrice{},
	}, map[string]any{}, []JSONOp{{
		Op: "json_set", Path: "$.provider.max_price.prompt", ValueExpr: "$model_channel.max_price.prompt",
	}})
	if err == nil {
		t.Fatal("expected missing model channel max price error")
	}
	if !strings.Contains(err.Error(), "$model_channel.max_price.prompt") {
		t.Fatalf("error=%q, want variable name", err)
	}
}

func TestApplyJSONOps_ModelChannelMaxPricePartiallyConfiguredReturnsError(t *testing.T) {
	prompt := 1.5
	_, err := ApplyJSONOps(&dslmeta.Meta{
		ModelChannelMaxPrice: &dslmeta.ModelChannelMaxPrice{Prompt: &prompt},
	}, map[string]any{}, []JSONOp{
		{Op: "json_set", Path: "$.provider.max_price.prompt", ValueExpr: "$model_channel.max_price.prompt"},
		{Op: "json_set", Path: "$.provider.max_price.completion", ValueExpr: "$model_channel.max_price.completion"},
	})
	if err == nil {
		t.Fatal("expected missing completion max price error")
	}
	if !strings.Contains(err.Error(), "$model_channel.max_price.completion") {
		t.Fatalf("error=%q, want missing completion variable", err)
	}
}

func TestApplyJSONOps_RequestModelNumericStringRemainsString(t *testing.T) {
	root, err := ApplyJSONOps(&dslmeta.Meta{OriginModelName: "123"}, map[string]any{}, []JSONOp{{
		Op: "json_set", Path: "$.model", ValueExpr: "$request.model",
	}})
	if err != nil {
		t.Fatalf("ApplyJSONOps: %v", err)
	}
	if got, ok := root["model"].(string); !ok || got != "123" {
		t.Fatalf("model=%v (%T), want string 123", root["model"], root["model"])
	}
}

func TestApplyJSONOps_TableDriven(t *testing.T) {
	t.Parallel()

	meta := &dslmeta.Meta{
		API:             "chat.completions",
		OriginModelName: "gpt-4o-mini",
		RequestHeaders: http.Header{
			"Anthropic-Beta": []string{" computer-use-2025-01-24 , unknown , CONTEXT-MANAGEMENT-2025-06-27 "},
		},
	}

	cases := []struct {
		name    string
		in      map[string]any
		ops     []JSONOp
		want    any
		wantErr bool
	}{
		{
			name: "json_set_nested_creates_objects",
			in:   map[string]any{"a": 1},
			ops: []JSONOp{
				{Op: "json_set", Path: "$.stream_options.include_usage", ValueExpr: "true"},
			},
			want: map[string]any{"a": 1, "stream_options": map[string]any{"include_usage": true}},
		},
		{
			name: "json_set_if_absent_sets_when_missing",
			in:   map[string]any{"a": 1},
			ops: []JSONOp{
				{Op: "json_set_if_absent", Path: "$.instructions", ValueExpr: "\"\""},
			},
			want: map[string]any{"a": 1, "instructions": ""},
		},
		{
			name: "json_set_if_absent_skips_when_present",
			in:   map[string]any{"instructions": "keep-me"},
			ops: []JSONOp{
				{Op: "json_set_if_absent", Path: "$.instructions", ValueExpr: "\"\""},
			},
			want: map[string]any{"instructions": "keep-me"},
		},
		{
			name: "json_replace_updates_existing_path",
			in:   map[string]any{"model": "upstream"},
			ops: []JSONOp{
				{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model"},
			},
			want: map[string]any{"model": "gpt-4o-mini"},
		},
		{
			name: "json_set_template_value",
			in:   map[string]any{"model": "upstream"},
			ops: []JSONOp{
				{Op: "json_set", Path: "$.route", ValueExpr: `template("/v1/${request.model}")`},
			},
			want: map[string]any{"model": "upstream", "route": "/v1/gpt-4o-mini"},
		},
		{
			name: "json_replace_missing_path_is_noop",
			in:   map[string]any{"a": 1},
			ops: []JSONOp{
				{Op: "json_replace", Path: "$.message.model", ValueExpr: "\"meta\""},
			},
			want: map[string]any{"a": 1},
		},
		{
			name: "json_del_missing_is_ok",
			in:   map[string]any{"a": 1},
			ops:  []JSONOp{{Op: "json_del", Path: "$.nope"}},
			want: map[string]any{"a": 1},
		},
		{
			name: "json_rename_moves_value",
			in:   map[string]any{"a": map[string]any{"b": 2}},
			ops:  []JSONOp{{Op: "json_rename", FromPath: "$.a.b", ToPath: "$.x.y"}},
			want: map[string]any{"a": map[string]any{}, "x": map[string]any{"y": 2}},
		},
		{
			name: "json_wrap_input_text_wraps_string",
			in: map[string]any{
				"input": "Generate an image of gray tabby cat hugging an otter with an orange scarf",
			},
			ops: []JSONOp{{Op: "json_wrap_input_text", Path: "$.input"}},
			want: map[string]any{
				"input": []any{
					map[string]any{
						"role": "user",
						"content": []any{
							map[string]any{
								"type": "input_text",
								"text": "Generate an image of gray tabby cat hugging an otter with an orange scarf",
							},
						},
					},
				},
			},
		},
		{
			name: "json_wrap_input_text_missing_path_is_noop",
			in:   map[string]any{"model": "gpt-image-1"},
			ops:  []JSONOp{{Op: "json_wrap_input_text", Path: "$.input"}},
			want: map[string]any{"model": "gpt-image-1"},
		},
		{
			name: "json_wrap_input_text_array_is_noop",
			in: map[string]any{
				"input": []any{
					map[string]any{"role": "user", "content": "already wrapped"},
				},
			},
			ops: []JSONOp{{Op: "json_wrap_input_text", Path: "$.input"}},
			want: map[string]any{
				"input": []any{
					map[string]any{"role": "user", "content": "already wrapped"},
				},
			},
		},
		{
			name: "json_keep_values_keeps_matching_items",
			in:   map[string]any{"model": "claude"},
			ops: []JSONOp{
				{
					Op:         "json_set_header_values",
					Path:       "$.anthropic_beta",
					HeaderName: "anthropic-beta",
				},
				{
					Op:   "json_keep_values",
					Path: "$.anthropic_beta",
					Patterns: []string{
						"computer-use-2025-01-24",
						"context-management-2025-06-27",
					},
				},
			},
			want: map[string]any{
				"model":          "claude",
				"anthropic_beta": []string{"computer-use-2025-01-24", "CONTEXT-MANAGEMENT-2025-06-27"},
			},
		},
		{
			name: "json_keep_values_wildcard_pattern",
			in:   map[string]any{"flags": []string{"computer-use-2025-01-24", "unknown", "context-management-2025-06-27", "beta-feature"}},
			ops: []JSONOp{
				{Op: "json_keep_values", Path: "$.flags", Patterns: []string{"computer-use-*", "context-management-*"}},
			},
			want: map[string]any{
				"flags": []string{"computer-use-2025-01-24", "context-management-2025-06-27"},
			},
		},
		{
			name: "json_keep_values_deletes_field_when_none_match",
			in:   map[string]any{"flags": []string{"unknown", "other"}},
			ops: []JSONOp{
				{Op: "json_keep_values", Path: "$.flags", Patterns: []string{"computer-use-*"}},
			},
			want: map[string]any{},
		},
		{
			name: "json_keep_values_case_insensitive_matching",
			in:   map[string]any{"flags": []string{"COMPUTER-USE-2025-01-24", "Other"}},
			ops: []JSONOp{
				{Op: "json_keep_values", Path: "$.flags", Patterns: []string{"computer-use-*"}},
			},
			want: map[string]any{"flags": []string{"COMPUTER-USE-2025-01-24"}},
		},
		// json_filter_values keeps matching values for now (backward compat with allowlist semantics).
		// TODO: After release, uncomment the denylist tests below and remove this keep-matching test.
		{
			name: "json_filter_values_keeps_matching_items",
			in:   map[string]any{"anthropic_beta": []string{"computer-use-2025-01-24", "unknown", "context-management-2025-06-27"}},
			ops: []JSONOp{
				{
					Op:   "json_filter_values",
					Path: "$.anthropic_beta",
					Patterns: []string{
						"computer-use-2025-01-24",
						"context-management-2025-06-27",
					},
				},
			},
			want: map[string]any{
				"anthropic_beta": []string{"computer-use-2025-01-24", "context-management-2025-06-27"},
			},
		},
		// TODO: After release, uncomment when json_filter_values switches to denylist semantics:
		// {
		// 	name: "json_filter_values_removes_matching_items",
		// 	in:   map[string]any{"anthropic_beta": []string{"computer-use-2025-01-24", "unknown", "context-management-2025-06-27"}},
		// 	ops: []JSONOp{{Op: "json_filter_values", Path: "$.anthropic_beta", Patterns: []string{"computer-use-2025-01-24", "context-management-2025-06-27"}}},
		// 	want: map[string]any{"anthropic_beta": []string{"unknown"}},
		// },
		// {
		// 	name: "json_filter_values_deletes_field_when_all_removed",
		// 	in:   map[string]any{"anthropic_beta": []string{"computer-use-2025-01-24"}},
		// 	ops:  []JSONOp{{Op: "json_filter_values", Path: "$.anthropic_beta", Patterns: []string{"computer-use-2025-01-24"}}},
		// 	want: map[string]any{},
		// },
		{
			name: "json_del_with_condition_filters_tools_and_tool_choice",
			in: map[string]any{
				"tools": []any{
					map[string]any{"type": "web_search_20260209", "name": "web_search"},
					map[string]any{"type": "custom", "name": "keep"},
					map[string]any{"type": "web_fetch_20250101", "name": "web_fetch"},
				},
				"tool_choice": map[string]any{"type": "web_search_20260209", "name": "web_search"},
			},
			ops: []JSONOp{
				{Op: "json_del_with_condition", Path: "$.tools", FieldName: "type", Patterns: []string{"web_search*", "web_fetch*"}},
				{Op: "json_del_with_condition", Path: "$.tool_choice", FieldName: "type", Patterns: []string{"web_search*", "web_fetch*"}},
			},
			want: map[string]any{
				"tools": []any{
					map[string]any{"type": "custom", "name": "keep"},
				},
			},
		},
		{
			name: "json_del_with_condition_deletes_empty_tools",
			in: map[string]any{
				"tools": []any{
					map[string]any{"type": "web_search_20260209", "name": "web_search"},
				},
			},
			ops:  []JSONOp{{Op: "json_del_with_condition", Path: "$.tools", FieldName: "type", Patterns: []string{"web_search*"}}},
			want: map[string]any{},
		},
		{
			name: "json_del_if_missing_deletes_tool_choice_after_tools_removed",
			in: map[string]any{
				"tools":       []any{map[string]any{"type": "web_search_20260209", "name": "web_search"}},
				"tool_choice": "auto",
			},
			ops: []JSONOp{
				{Op: "json_del_with_condition", Path: "$.tools", FieldName: "type", Patterns: []string{"web_search*"}},
				{Op: "json_del_if_missing", Path: "$.tool_choice", FromPath: "$.tools"},
			},
			want: map[string]any{},
		},
		{
			name: "json_del_if_missing_keeps_tool_choice_when_tools_remain",
			in: map[string]any{
				"tools": []any{
					map[string]any{"type": "web_search_20260209", "name": "web_search"},
					map[string]any{"type": "custom", "name": "keep"},
				},
				"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "keep"}},
			},
			ops: []JSONOp{
				{Op: "json_del_with_condition", Path: "$.tools", FieldName: "type", Patterns: []string{"web_search*"}},
				{Op: "json_del_if_missing", Path: "$.tool_choice", FromPath: "$.tools"},
			},
			want: map[string]any{
				"tools":       []any{map[string]any{"type": "custom", "name": "keep"}},
				"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "keep"}},
			},
		},
		{
			name: "json_del_with_condition_ignores_scalar",
			in:   map[string]any{"tool_choice": "auto"},
			ops:  []JSONOp{{Op: "json_del_with_condition", Path: "$.tool_choice", FieldName: "type", Patterns: []string{"web_search*", "web_fetch*"}}},
			want: map[string]any{"tool_choice": "auto"},
		},
		{
			name:    "json_wrap_input_text_rejects_object",
			in:      map[string]any{"input": map[string]any{"text": "bad"}},
			ops:     []JSONOp{{Op: "json_wrap_input_text", Path: "$.input"}},
			wantErr: true,
		},
		{
			name:    "invalid_path_prefix",
			in:      map[string]any{"a": 1},
			ops:     []JSONOp{{Op: "json_set", Path: "a.b", ValueExpr: "true"}},
			wantErr: true,
		},
		{
			name:    "array_index_not_supported",
			in:      map[string]any{"a": 1},
			ops:     []JSONOp{{Op: "json_set", Path: "$.a[0]", ValueExpr: "true"}},
			wantErr: true,
		},
		{name: "nil_root", in: nil, ops: []JSONOp{{Op: "json_set", Path: "$.a", ValueExpr: "true"}}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ApplyJSONOps(meta, tc.in, tc.ops)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got=%#v want=%#v", got, tc.want)
			}
		})
	}
}
