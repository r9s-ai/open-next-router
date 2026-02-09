package jsonutil

import "testing"

func TestGetIntByPath_SupportsWildcardAndIndex(t *testing.T) {
	root := map[string]any{
		"usage": map[string]any{
			"items": []any{
				map[string]any{"tokens": 2},
				map[string]any{"tokens": 3},
			},
			"first": []any{
				map[string]any{"v": 7},
			},
		},
	}

	if got := GetIntByPath(root, "$.usage.items[*].tokens"); got != 5 {
		t.Fatalf("wildcard sum got %d, want 5", got)
	}
	if got := GetIntByPath(root, "$.usage.first[0].v"); got != 7 {
		t.Fatalf("index access got %d, want 7", got)
	}
}

func TestCoerceInt_StringAndArray(t *testing.T) {
	if got := CoerceInt("12"); got != 12 {
		t.Fatalf("string cast got %d, want 12", got)
	}
	if got := CoerceInt([]any{1, float64(2), "3"}); got != 6 {
		t.Fatalf("array sum got %d, want 6", got)
	}
}
