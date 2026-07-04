package dslconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func TestParseJSONMapValueAndScale(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_map_value "$.voice" "alloy" "male-qn-qingse";
      json_map_value "$.voice" "nova" "female-shaonv";
      json_scale "$.speed" in_min=0.25 in_max=4.0 out_min=0.5 out_max=2.0;
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
	scale := ops[2]
	if scale.Op != jsonOpScale || scale.ScaleRange == nil {
		t.Fatalf("unexpected scale op: %#v", scale)
	}
	if scale.ScaleRange.InMin != 0.25 || scale.ScaleRange.InMax != 4.0 || scale.ScaleRange.OutMin != 0.5 || scale.ScaleRange.OutMax != 2.0 {
		t.Fatalf("unexpected scale range: %#v", scale.ScaleRange)
	}
}

func TestParseJSONScaleRejectsMissingOption(t *testing.T) {
	conf := `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" {
    request {
      json_scale "$.speed" in_min=0.25 in_max=4.0 out_min=0.5;
    }
  }
}`
	_, _, _, _, _, _, _, _, _, err := parseProviderConfig("demo.conf", conf)
	if err == nil {
		t.Fatalf("expected error for missing out_max")
	}
}

func TestApplyJSONOps_MapValueAndScale(t *testing.T) {
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
			name: "scale_linear_mid",
			in:   map[string]any{"speed": 1.0},
			ops:  []JSONOp{{Op: jsonOpScale, Path: "$.speed", ScaleRange: &JSONScaleRange{InMin: 0.25, InMax: 4.0, OutMin: 0.5, OutMax: 2.0}}},
			key:  "speed",
			want: 0.5 + (1.0-0.25)*1.5/3.75,
		},
		{
			name: "scale_clamps_above",
			in:   map[string]any{"speed": 9.5},
			ops:  []JSONOp{{Op: jsonOpScale, Path: "$.speed", ScaleRange: &JSONScaleRange{InMin: 0.25, InMax: 4.0, OutMin: 0.5, OutMax: 2.0}}},
			key:  "speed",
			want: 2.0,
		},
		{
			name: "scale_missing_field_noop",
			in:   map[string]any{"other": 1},
			ops:  []JSONOp{{Op: jsonOpScale, Path: "$.speed", ScaleRange: &JSONScaleRange{InMin: 0.25, InMax: 4.0, OutMin: 0.5, OutMax: 2.0}}},
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
      json_scale "$.speed" in_min=0.25 in_max=4.0 out_min=0.5 out_max=2.0;
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
