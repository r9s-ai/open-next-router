package dslconfig

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

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
			name: "json_set_header_values_sets_header_items_then_filter_values_filters",
			in:   map[string]any{"model": "claude"},
			ops: []JSONOp{
				{
					Op:         "json_set_header_values",
					Path:       "$.anthropic_beta",
					HeaderName: "anthropic-beta",
				},
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
				"model":          "claude",
				"anthropic_beta": []string{"computer-use-2025-01-24", "CONTEXT-MANAGEMENT-2025-06-27"},
			},
		},
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
