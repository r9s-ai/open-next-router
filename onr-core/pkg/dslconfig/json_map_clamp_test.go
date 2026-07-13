package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestParseJSONMapValueAndClamp(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" "alloy" "male-qn-qingse";
      json_map_value "$.voice" "nova" "female-shaonv";
      json_clamp "$.speed" min=0.5 max=2.0;
    }
  }
}`
	_, _, req, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(req.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(req.Matches))
	}
	ops := req.Matches[0].Transform.JSONOps
	if len(ops) != 3 {
		t.Fatalf("expected 3 json ops, got %d", len(ops))
	}
	if ops[0].Op != jsonOpMapValue || ops[0].Path != "$.voice" || ops[0].MatchValue != "alloy" || ops[0].ValueExpr != `"male-qn-qingse"` {
		t.Fatalf("unexpected map value op: %#v", ops[0])
	}
	clamp := ops[2]
	if clamp.Op != jsonOpClamp || clamp.ClampRange == nil {
		t.Fatalf("unexpected clamp op: %#v", clamp)
	}
	if clamp.ClampRange.Min != 0.5 || clamp.ClampRange.Max != 2.0 {
		t.Fatalf("unexpected clamp range: %#v", clamp.ClampRange)
	}
}

func TestParseJSONMapValueBlockExpandsToOps(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" {
        "alloy" "male-qn-qingse";
        "echo"  "Deep_Voice_Man";
        "nova"  "Lively_Girl";
      }
      json_rename "$.voice" "$.voice_setting.voice_id";
    }
  }
}`
	_, _, req, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ops := req.Matches[0].Transform.JSONOps
	// 3 expanded map ops + 1 rename
	if len(ops) != 4 {
		t.Fatalf("expected 4 ops, got %d: %#v", len(ops), ops)
	}
	want := []struct{ from, to string }{
		{"alloy", `"male-qn-qingse"`},
		{"echo", `"Deep_Voice_Man"`},
		{"nova", `"Lively_Girl"`},
	}
	for i, w := range want {
		op := ops[i]
		if op.Op != jsonOpMapValue || op.Path != "$.voice" || op.MatchValue != w.from || op.ValueExpr != w.to {
			t.Fatalf("op[%d] unexpected: %#v", i, op)
		}
	}
	if ops[3].Op != jsonOpRename {
		t.Fatalf("expected rename op last, got %#v", ops[3])
	}
}

func TestParseJSONMapValueBlockRejectsEmpty(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" {
      }
    }
  }
}`
	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected error for empty json_map_value block")
	}
}

// Block and single forms must produce identical ops.
func TestParseJSONMapValueBlockEqualsSingle(t *testing.T) {
	blockConf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request { json_map_value "$.voice" { "alloy" "male-qn-qingse"; "nova" "Lively_Girl"; } }
  }
}`
	singleConf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" "alloy" "male-qn-qingse";
      json_map_value "$.voice" "nova" "Lively_Girl";
    }
  }
}`
	_, _, rb, _, _, _, _, _, _, err := parseProviderConfig("b.conf", blockConf)
	if err != nil {
		t.Fatalf("parse block: %v", err)
	}
	_, _, rs, _, _, _, _, _, _, err := parseProviderConfig("s.conf", singleConf)
	if err != nil {
		t.Fatalf("parse single: %v", err)
	}
	ob, os := rb.Matches[0].Transform.JSONOps, rs.Matches[0].Transform.JSONOps
	if len(ob) != len(os) {
		t.Fatalf("op count differs: block=%d single=%d", len(ob), len(os))
	}
	for i := range ob {
		if ob[i].Op != os[i].Op || ob[i].Path != os[i].Path || ob[i].MatchValue != os[i].MatchValue || ob[i].ValueExpr != os[i].ValueExpr {
			t.Fatalf("op[%d] differs: block=%#v single=%#v", i, ob[i], os[i])
		}
	}
}

func TestParseJSONClampRejectsMissingOption(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_clamp "$.speed" min=0.5;
    }
  }
}`
	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected error for missing max")
	}
}

func TestApplyJSONOps_MapValueAndClamp(t *testing.T) {
	t.Parallel()
	m := &dslmeta.Meta{API: "audio.speech", OriginModelName: "tts-1"}

	cases := []struct {
		name string
		in   map[string]any
		ops  []JSONOp
		key  string
		want any
	}{
		{
			name: "map_value_hit",
			in:   map[string]any{"voice": "alloy"},
			ops:  []JSONOp{{Op: jsonOpMapValue, Path: "$.voice", MatchValue: "alloy", ValueExpr: `"male-qn-qingse"`}},
			key:  "voice",
			want: "male-qn-qingse",
		},
		{
			name: "map_value_miss_keeps_original",
			in:   map[string]any{"voice": "native-voice-300"},
			ops:  []JSONOp{{Op: jsonOpMapValue, Path: "$.voice", MatchValue: "alloy", ValueExpr: `"male-qn-qingse"`}},
			key:  "voice",
			want: "native-voice-300",
		},
		{
			name: "clamp_in_range_passthrough",
			in:   map[string]any{"speed": 1.0},
			ops:  []JSONOp{{Op: jsonOpClamp, Path: "$.speed", ClampRange: &JSONClampRange{Min: 0.5, Max: 2.0}}},
			key:  "speed",
			want: 1.0,
		},
		{
			name: "clamp_below_min",
			in:   map[string]any{"speed": 0.25},
			ops:  []JSONOp{{Op: jsonOpClamp, Path: "$.speed", ClampRange: &JSONClampRange{Min: 0.5, Max: 2.0}}},
			key:  "speed",
			want: 0.5,
		},
		{
			name: "clamp_above_max",
			in:   map[string]any{"speed": 4.0},
			ops:  []JSONOp{{Op: jsonOpClamp, Path: "$.speed", ClampRange: &JSONClampRange{Min: 0.5, Max: 2.0}}},
			key:  "speed",
			want: 2.0,
		},
		{
			name: "clamp_missing_field_noop",
			in:   map[string]any{"other": 1},
			ops:  []JSONOp{{Op: jsonOpClamp, Path: "$.speed", ClampRange: &JSONClampRange{Min: 0.5, Max: 2.0}}},
			key:  "speed",
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ApplyJSONOps(m, tc.in, tc.ops)
			if err != nil {
				t.Fatalf("ApplyJSONOps: %v", err)
			}
			if got := out[tc.key]; got != tc.want {
				t.Fatalf("expected %v, got %#v", tc.want, got)
			}
		})
	}
}

func TestEvalJSONValueExprFloatLiteral(t *testing.T) {
	t.Parallel()
	m := &dslmeta.Meta{}
	if got := evalJSONValueExpr(m, "1.0"); got != 1.0 {
		t.Fatalf("expected float 1.0, got %#v", got)
	}
	if got := evalJSONValueExpr(m, "-0.5"); got != -0.5 {
		t.Fatalf("expected float -0.5, got %#v", got)
	}
	// ints stay ints
	if got := evalJSONValueExpr(m, "32000"); got != 32000 {
		t.Fatalf("expected int 32000, got %#v", got)
	}
	// words never become floats
	if got := evalJSONValueExpr(m, "infe"); got == float64(0) {
		t.Fatalf("expected non-float for word, got %#v", got)
	}
}

func TestValidateProviderFileAcceptsNewJSONOpsAndFloats(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  defaults {
    upstream_config { base_url = "https://api.example.com"; }
  }
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" "alloy" "male-qn-qingse";
      json_clamp "$.speed" min=0.5 max=2.0;
      json_set_if_absent "$.voice_setting.speed" 1.0;
    }
    upstream { set_path "/v1/t2a_v2"; }
  }
}`
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatalf("write conf: %v", err)
	}
	if _, err := ValidateProviderFile(path); err != nil {
		t.Fatalf("ValidateProviderFile: %v", err)
	}
}
