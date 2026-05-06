package apitypes

import "testing"

func TestMapListValue_CopiesInnerMaps(t *testing.T) {
	in := map[string]any{
		"items": []any{
			map[string]any{"k": "v"},
		},
	}
	out, err := mapListValue(in, "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("unexpected output length: %d", len(out))
	}
	out[0]["k"] = "changed"
	rawItems, ok := in["items"].([]any)
	if !ok || len(rawItems) != 1 {
		t.Fatalf("unexpected original items: %#v", in["items"])
	}
	inner, ok := rawItems[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected original inner item: %#v", rawItems[0])
	}
	if inner["k"] != "v" {
		t.Fatalf("input map mutated, got %v", inner["k"])
	}
}

func TestMapStringAnySliceValue_CopiesInnerMaps(t *testing.T) {
	in := map[string]any{
		"items": []any{
			map[string]any{"k": "v"},
		},
	}
	out, err := mapStringAnySliceValue(in, "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("unexpected output length: %d", len(out))
	}
	out[0]["k"] = "changed"
	rawItems, ok := in["items"].([]any)
	if !ok || len(rawItems) != 1 {
		t.Fatalf("unexpected original items: %#v", in["items"])
	}
	inner, ok := rawItems[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected original inner item: %#v", rawItems[0])
	}
	if inner["k"] != "v" {
		t.Fatalf("input map mutated, got %v", inner["k"])
	}
}

func TestMapListValue_ShallowCopyNestedMapShared(t *testing.T) {
	nested := map[string]any{"x": 1}
	in := map[string]any{
		"items": []any{
			map[string]any{"nested": nested},
		},
	}
	out, err := mapListValue(in, "items")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotNested, ok := out[0]["nested"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected nested value: %#v", out[0]["nested"])
	}
	gotNested["x"] = 2
	if nested["x"] != 2 {
		t.Fatalf("expected shallow copy for nested map")
	}
}
