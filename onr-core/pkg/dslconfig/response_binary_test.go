package dslconfig

import (
	"encoding/hex"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

const binaryDirectivesConf = `syntax "next-router/0.1";
provider "demo" {
  match api = "audio.speech" stream = false {
    error {
      error_map openai;
      error_when path="$.base_resp.status_code" ne=0 status=400;
    }
    response {
      resp_body_extract path="$.data.audio" decode=hex;
      resp_content_type from_path="$.extra_info.audio_format" kind=audio default="mp3";
    }
  }
  match api = "audio.speech" stream = true {
    response {
      sse_binary_extract path="$.data.audio" decode=hex stop_path="$.data.status" stop_eq=2;
    }
  }
}`

func TestParseResponseBinaryDirectives(t *testing.T) {
	_, _, _, resp, perr, _, _, _, _, err := parseProviderConfig("demo.conf", binaryDirectivesConf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(resp.Matches) != 2 {
		t.Fatalf("expected 2 response matches, got %d", len(resp.Matches))
	}

	nonStream := resp.Matches[0].Response
	if nonStream.BodyExtract == nil || nonStream.BodyExtract.Path != "$.data.audio" || nonStream.BodyExtract.Decode != "hex" {
		t.Fatalf("unexpected body extract: %#v", nonStream.BodyExtract)
	}
	if nonStream.ContentTypeRule == nil || nonStream.ContentTypeRule.FromPath != "$.extra_info.audio_format" || nonStream.ContentTypeRule.Kind != "audio" || nonStream.ContentTypeRule.Default != "mp3" {
		t.Fatalf("unexpected content type rule: %#v", nonStream.ContentTypeRule)
	}

	stream := resp.Matches[1].Response
	if stream.SSEBinaryExtract == nil || stream.SSEBinaryExtract.Path != "$.data.audio" || stream.SSEBinaryExtract.StopPath != "$.data.status" || stream.SSEBinaryExtract.StopEquals != "2" {
		t.Fatalf("unexpected sse binary extract: %#v", stream.SSEBinaryExtract)
	}

	errRules := perr.Matches[0].Response.ErrorWhen
	if len(errRules) != 1 {
		t.Fatalf("expected 1 error_when rule, got %d", len(errRules))
	}
	rule := errRules[0]
	if rule.Path != "$.base_resp.status_code" || rule.NotEquals != "0" || rule.Status != 400 {
		t.Fatalf("unexpected error_when rule: %#v", rule)
	}
}

func TestResponseDirectiveSelectWithBinaryDirectives(t *testing.T) {
	_, _, _, resp, _, _, _, _, _, err := parseProviderConfig("demo.conf", binaryDirectivesConf)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	dir, ok := resp.Select(&dslmeta.Meta{API: "audio.speech", IsStream: false})
	if !ok || dir.BodyExtract == nil {
		t.Fatalf("expected non-stream directive with body extract, got ok=%v dir=%#v", ok, dir)
	}
	dir, ok = resp.Select(&dslmeta.Meta{API: "audio.speech", IsStream: true})
	if !ok || dir.SSEBinaryExtract == nil {
		t.Fatalf("expected stream directive with sse binary extract, got ok=%v dir=%#v", ok, dir)
	}
}

func TestRespBodyExtractRule_ExtractBody(t *testing.T) {
	audio := []byte{0x49, 0x44, 0x33, 0x04}
	root := map[string]any{
		"data":       map[string]any{"audio": hex.EncodeToString(audio), "status": 2},
		"extra_info": map[string]any{"audio_format": "wav"},
	}
	rule := &RespBodyExtractRule{Path: "$.data.audio", Decode: "hex"}
	got, err := rule.ExtractBody(root)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if string(got) != string(audio) {
		t.Fatalf("unexpected body: %x", got)
	}

	ct := &RespContentTypeRule{FromPath: "$.extra_info.audio_format", Kind: "audio", Default: "mp3"}
	if mime := ct.ResolveContentType(root, ""); mime != "audio/wav" {
		t.Fatalf("expected audio/wav, got %s", mime)
	}
	if mime := ct.ResolveContentType(map[string]any{}, ""); mime != "audio/mpeg" {
		t.Fatalf("expected default audio/mpeg, got %s", mime)
	}

	if _, err := rule.ExtractBody(map[string]any{"data": map[string]any{}}); err == nil {
		t.Fatalf("expected error for missing audio field")
	}
}

func TestSSEBinaryExtractRule_DecodeChunk(t *testing.T) {
	rule := &SSEBinaryExtractRule{Path: "$.data.audio", Decode: "hex", StopPath: "$.data.status", StopEquals: "2"}

	data, stop, err := rule.DecodeChunk(map[string]any{"data": map[string]any{"audio": "4944", "status": 1}})
	if err != nil || stop {
		t.Fatalf("unexpected err=%v stop=%v", err, stop)
	}
	if string(data) != string([]byte{0x49, 0x44}) {
		t.Fatalf("unexpected chunk: %x", data)
	}

	data, stop, err = rule.DecodeChunk(map[string]any{"data": map[string]any{"audio": "", "status": 2}})
	if err != nil || !stop || data != nil {
		t.Fatalf("expected stop with no data, got data=%x stop=%v err=%v", data, stop, err)
	}

	if _, _, err := rule.DecodeChunk(map[string]any{"data": map[string]any{"audio": "zz", "status": 1}}); err == nil {
		t.Fatalf("expected decode error for invalid hex")
	}
}

func TestErrorWhenRule_Matches(t *testing.T) {
	ne := ErrorWhenRule{Path: "$.base_resp.status_code", NotEquals: "0", Status: 400}
	if ne.Matches(map[string]any{"base_resp": map[string]any{"status_code": 0}}) {
		t.Fatalf("status_code 0 must not match ne=0")
	}
	if !ne.Matches(map[string]any{"base_resp": map[string]any{"status_code": 1004}}) {
		t.Fatalf("status_code 1004 must match ne=0")
	}
	// 缺失路径不触发误报
	if ne.Matches(map[string]any{"data": map[string]any{}}) {
		t.Fatalf("missing path must not match ne rule")
	}

	eq := ErrorWhenRule{Path: "$.status", Equals: "error", Status: 400}
	if !eq.Matches(map[string]any{"status": "error"}) {
		t.Fatalf("status error must match eq rule")
	}
	if eq.Matches(map[string]any{"status": "ok"}) {
		t.Fatalf("status ok must not match eq rule")
	}
}
