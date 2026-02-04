package dslconfig

import (
	"reflect"
	"testing"

	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
)

func TestApplyJSONOps_TableDriven(t *testing.T) {
	t.Parallel()

	meta := &dslmeta.Meta{API: "chat.completions"}

	cases := []struct {
		name    string
		in      any
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
			want: map[string]any{"a": float64(1), "stream_options": map[string]any{"include_usage": true}},
		},
		{
			name: "json_del_missing_is_ok",
			in:   map[string]any{"a": 1},
			ops:  []JSONOp{{Op: "json_del", Path: "$.nope"}},
			want: map[string]any{"a": float64(1)},
		},
		{
			name: "json_rename_moves_value",
			in:   map[string]any{"a": map[string]any{"b": 2}},
			ops:  []JSONOp{{Op: "json_rename", FromPath: "$.a.b", ToPath: "$.x.y"}},
			want: map[string]any{"a": map[string]any{}, "x": map[string]any{"y": float64(2)}},
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
		{
			name:    "root_not_object",
			in:      []any{"x"},
			ops:     []JSONOp{{Op: "json_set", Path: "$.a", ValueExpr: "true"}},
			wantErr: true,
		},
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
