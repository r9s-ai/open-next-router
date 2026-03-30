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

func TestGetFirstIntByPaths(t *testing.T) {
	root := map[string]any{
		"usage": map[string]any{
			"prompt_tokens": 5,
			"input_tokens":  9,
		},
	}

	if got := GetFirstIntByPaths(root, "$.usage.prompt_tokens", "$.usage.input_tokens"); got != 5 {
		t.Fatalf("first int got %d, want 5", got)
	}
	if got := GetFirstIntByPaths(root, "$.usage.missing", "$.usage.input_tokens"); got != 9 {
		t.Fatalf("fallback int got %d, want 9", got)
	}
}

func TestGetStringByPath_SupportsWildcardAndIndex(t *testing.T) {
	root := map[string]any{
		"items": []any{
			map[string]any{"v": ""},
			map[string]any{"v": "x"},
		},
		"one": []any{
			map[string]any{"name": "first"},
		},
	}

	if got := GetStringByPath(root, "$.items[*].v"); got != "x" {
		t.Fatalf("wildcard string got %q, want x", got)
	}
	if got := GetStringByPath(root, "$.one[0].name"); got != "first" {
		t.Fatalf("index string got %q, want first", got)
	}
}

func TestGetFirstStringByPaths(t *testing.T) {
	root := map[string]any{
		"delta": map[string]any{
			"stop_reason": "",
		},
		"message": map[string]any{
			"stop_reason": "end_turn",
		},
	}

	if got := GetFirstStringByPaths(root, "$.delta.stop_reason", "$.message.stop_reason"); got != "end_turn" {
		t.Fatalf("first string got %q, want end_turn", got)
	}
}

func TestGetFloatByPath_StringAndNumber(t *testing.T) {
	root := map[string]any{
		"usage": map[string]any{
			"a": 1.5,
			"b": "2.5",
		},
	}

	if got := GetFloatByPath(root, "$.usage.a"); got != 1.5 {
		t.Fatalf("float value got %v, want 1.5", got)
	}
	if got := GetFloatByPath(root, "$.usage.b"); got != 2.5 {
		t.Fatalf("string float got %v, want 2.5", got)
	}
}

func TestGetFloatByPathWithMatch(t *testing.T) {
	root := map[string]any{
		"usage": map[string]any{
			"items": []any{
				map[string]any{"tokens": 1.5},
				map[string]any{"tokens": "2.5"},
			},
			"empty": []any{},
		},
	}

	if got, matched := GetFloatByPathWithMatch(root, "$.usage.items[*].tokens"); !matched || got != 4 {
		t.Fatalf("wildcard float got (%v, %v), want (4, true)", got, matched)
	}
	if got, matched := GetFloatByPathWithMatch(root, "$.usage.empty[*].tokens"); !matched || got != 0 {
		t.Fatalf("empty wildcard got (%v, %v), want (0, true)", got, matched)
	}
	if got, matched := GetFloatByPathWithMatch(root, "$.usage.missing"); matched || got != 0 {
		t.Fatalf("missing path got (%v, %v), want (0, false)", got, matched)
	}
}

func TestVisitValuesByPath_EmptyWildcardStillMatches(t *testing.T) {
	root := map[string]any{
		"usage": map[string]any{
			"items": []any{},
		},
	}

	visited := 0
	matched := VisitValuesByPath(root, "$.usage.items[*].tokens", func(v any) {
		visited++
	})
	if !matched {
		t.Fatalf("expected empty wildcard path to match")
	}
	if visited != 0 {
		t.Fatalf("visited got %d, want 0", visited)
	}
}
