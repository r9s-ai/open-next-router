package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRouter_Basic(t *testing.T) {
	r := NewRouter(map[string]Route{
		" gpt-4o-mini ": {
			Providers: []string{" OpenAI ", "azure", ""},
			OwnedBy:   "  me ",
		},
		"bad-empty": {
			Providers: nil,
		},
	})

	models := r.Models()
	if len(models) != 2 {
		t.Fatalf("models len=%d", len(models))
	}
	if models[0] != "bad-empty" || models[1] != "gpt-4o-mini" {
		t.Fatalf("unexpected models=%v", models)
	}

	p1, ok := r.NextProvider("gpt-4o-mini")
	if !ok || p1 != "openai" {
		t.Fatalf("next provider #1: %q %v", p1, ok)
	}
	p2, ok := r.NextProvider("gpt-4o-mini")
	if !ok || p2 != "azure" {
		t.Fatalf("next provider #2: %q %v", p2, ok)
	}
	p3, ok := r.NextProvider("gpt-4o-mini")
	if !ok || p3 != "openai" {
		t.Fatalf("next provider #3: %q %v", p3, ok)
	}

	if _, ok := r.NextProvider("unknown"); ok {
		t.Fatalf("unknown model should not match")
	}
	if _, ok := r.NextProvider("bad-empty"); ok {
		t.Fatalf("empty provider model should not match")
	}
}

func TestRouter_UnknownStrategyFallback(t *testing.T) {
	r := NewRouter(map[string]Route{
		"x": {
			Providers: []string{"a", "b"},
			Strategy:  Strategy("custom"),
		},
	})
	p1, _ := r.NextProvider("x")
	p2, _ := r.NextProvider("x")
	if p1 != "a" || p2 != "b" {
		t.Fatalf("unexpected fallback rr order: %q,%q", p1, p2)
	}
}

func TestRouter_EmptyRouter(t *testing.T) {
	r := NewRouter(nil)
	if got := r.Models(); len(got) != 0 {
		t.Fatalf("empty router models=%v", got)
	}
	if _, ok := r.NextProvider("x"); ok {
		t.Fatalf("empty router should not match")
	}
}

func TestToOpenAIList(t *testing.T) {
	r := NewRouter(map[string]Route{
		"a": {Providers: []string{"p"}},
	})
	v := r.ToOpenAIList()
	if v["object"] != "list" {
		t.Fatalf("unexpected object=%v", v["object"])
	}
	data, ok := v["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("unexpected data=%T %#v", v["data"], v["data"])
	}

	v2 := r.ToOpenAIListAt(1700000000)
	data2 := v2["data"].([]any)
	item, _ := data2[0].(map[string]any)
	if item["created"] != int64(1700000000) {
		t.Fatalf("unexpected created=%v", item["created"])
	}
}

func TestLoad(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		r, err := Load("")
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if len(r.Models()) != 0 {
			t.Fatalf("expected empty models")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		r, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if len(r.Models()) != 0 {
			t.Fatalf("expected empty models")
		}
	})

	t.Run("bad yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "models.yaml")
		if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := Load(path); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("ok yaml", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "models.yaml")
		if err := os.WriteFile(path, []byte(`
models:
  gpt-4o-mini:
    providers: [openai, azure]
`), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		r, err := Load(path)
		if err != nil {
			t.Fatalf("Load err=%v", err)
		}
		if _, ok := r.NextProvider("gpt-4o-mini"); !ok {
			t.Fatalf("expected model route")
		}
	})
}
