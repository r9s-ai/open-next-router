package dslconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/stretchr/testify/require"
)

func TestValidateProviderFile_ResponseJSONOpsAndSSEDelIf(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    response {
      json_del "$.usage";
      json_set "$.foo" "bar";
      json_replace "$.model" $request.model event="message_start" event_optional=true max_count=1;
      json_set_if_absent "$.bar" "baz";
      json_rename "$.a" "$.b";
      sse_json_del_if "$.type" "message_delta" "$.usage";
    }
  }
}
`), 0o600))

	pf, err := ValidateProviderFile(path)
	require.NoError(t, err)

	d := pf.Response.Defaults
	require.Len(t, d.JSONOps, 5)
	require.Equal(t, "json_del", d.JSONOps[0].Op)
	require.Equal(t, "$.usage", d.JSONOps[0].Path)
	require.Equal(t, "json_set", d.JSONOps[1].Op)
	require.Equal(t, "$.foo", d.JSONOps[1].Path)
	require.Equal(t, "\"bar\"", d.JSONOps[1].ValueExpr)
	require.Equal(t, "json_replace", d.JSONOps[2].Op)
	require.Equal(t, "$.model", d.JSONOps[2].Path)
	require.Equal(t, "$request.model", d.JSONOps[2].ValueExpr)
	require.Equal(t, "message_start", d.JSONOps[2].Event)
	require.True(t, d.JSONOps[2].EventOptional)
	require.Equal(t, 1, d.JSONOps[2].MaxCount)
	require.Equal(t, "json_set_if_absent", d.JSONOps[3].Op)
	require.Equal(t, "$.bar", d.JSONOps[3].Path)
	require.Equal(t, "\"baz\"", d.JSONOps[3].ValueExpr)
	require.Equal(t, "json_rename", d.JSONOps[4].Op)
	require.Equal(t, "$.a", d.JSONOps[4].FromPath)
	require.Equal(t, "$.b", d.JSONOps[4].ToPath)

	require.Len(t, d.SSEJSONDelIf, 1)
	require.Equal(t, "$.type", d.SSEJSONDelIf[0].CondPath)
	require.Equal(t, "message_delta", d.SSEJSONDelIf[0].Equals)
	require.Equal(t, "$.usage", d.SSEJSONDelIf[0].DelPath)
}

func TestTransformSSEEventDataJSON_EventAndMaxCount(t *testing.T) {
	in := "" +
		"event: message_start\n" +
		"data: {\"message\":{\"model\":\"upstream-1\"}}\n\n" +
		"event: message_delta\n" +
		"data: {\"usage\":{\"output_tokens\":1}}\n\n" +
		"event: message_start\n" +
		"data: {\"message\":{\"model\":\"upstream-2\"}}\n\n" +
		"data: [DONE]\n\n"

	ops := []JSONOp{
		{Op: "json_replace", Path: "$.message.model", ValueExpr: "$request.model", Event: "message_start", MaxCount: 1},
	}

	var out bytes.Buffer
	err := TransformSSEEventDataJSON(
		bytes.NewBufferString(in),
		&out,
		&dslmeta.Meta{OriginModelName: "meta-model"},
		nil,
		ops,
	)
	require.NoError(t, err)

	var payloads []map[string]any
	for _, ev := range bytes.Split(out.Bytes(), []byte("\n\n")) {
		for _, line := range bytes.Split(ev, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			raw := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(raw) == 0 || bytes.Equal(raw, []byte("[DONE]")) {
				continue
			}
			var obj map[string]any
			require.NoError(t, json.Unmarshal(raw, &obj))
			payloads = append(payloads, obj)
		}
	}
	require.Len(t, payloads, 3)

	msg0 := payloads[0]["message"].(map[string]any)
	require.Equal(t, "meta-model", msg0["model"])
	_, deltaHasMessage := payloads[1]["message"]
	require.False(t, deltaHasMessage)
	msg2 := payloads[2]["message"].(map[string]any)
	require.Equal(t, "upstream-2", msg2["model"])
}

func TestTransformSSEEventDataJSON_JSONOpsEventOptionalFallsBackWhenEventMissing(t *testing.T) {
	in := "" +
		"data: {\"message\":{\"model\":\"upstream-no-event\"},\"drop\":true,\"old\":\"legacy\"}\n\n" +
		"event: message_delta\n" +
		"data: {\"message\":{\"model\":\"upstream-delta\"},\"drop\":true,\"old\":\"legacy\"}\n\n" +
		"event: message_start\n" +
		"data: {\"message\":{\"model\":\"upstream-start\"},\"drop\":true,\"old\":\"legacy\"}\n\n" +
		"data: [DONE]\n\n"

	ops := []JSONOp{
		{Op: "json_replace", Path: "$.message.model", ValueExpr: "$request.model", Event: "message_start", EventOptional: true},
		{Op: "json_set", Path: "$.set_by_rule", ValueExpr: "true", Event: "message_start", EventOptional: true},
		{Op: "json_set_if_absent", Path: "$.message.role", ValueExpr: "\"assistant\"", Event: "message_start", EventOptional: true},
		{Op: "json_del", Path: "$.drop", Event: "message_start", EventOptional: true},
		{Op: "json_rename", FromPath: "$.old", ToPath: "$.new", Event: "message_start", EventOptional: true},
	}

	var out bytes.Buffer
	err := TransformSSEEventDataJSON(
		bytes.NewBufferString(in),
		&out,
		&dslmeta.Meta{OriginModelName: "meta-model"},
		nil,
		ops,
	)
	require.NoError(t, err)

	payloads := responseJSONPayloads(t, out.Bytes())
	require.Len(t, payloads, 3)

	msg0 := payloads[0]["message"].(map[string]any)
	require.Equal(t, "meta-model", msg0["model"])
	require.Equal(t, "assistant", msg0["role"])
	require.Equal(t, true, payloads[0]["set_by_rule"])
	_, hasDrop0 := payloads[0]["drop"]
	require.False(t, hasDrop0)
	require.Equal(t, "legacy", payloads[0]["new"])
	_, hasOld0 := payloads[0]["old"]
	require.False(t, hasOld0)

	msg1 := payloads[1]["message"].(map[string]any)
	require.Equal(t, "upstream-delta", msg1["model"])
	_, hasRole1 := msg1["role"]
	require.False(t, hasRole1)
	_, hasSet1 := payloads[1]["set_by_rule"]
	require.False(t, hasSet1)
	require.Equal(t, true, payloads[1]["drop"])
	require.Equal(t, "legacy", payloads[1]["old"])
	_, hasNew1 := payloads[1]["new"]
	require.False(t, hasNew1)

	msg2 := payloads[2]["message"].(map[string]any)
	require.Equal(t, "meta-model", msg2["model"])
	require.Equal(t, "assistant", msg2["role"])
	require.Equal(t, true, payloads[2]["set_by_rule"])
	_, hasDrop2 := payloads[2]["drop"]
	require.False(t, hasDrop2)
	require.Equal(t, "legacy", payloads[2]["new"])
	_, hasOld2 := payloads[2]["old"]
	require.False(t, hasOld2)
}

func TestValidateProviderFile_ResponseJSONReplaceEventOptionalRequiresEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    response {
      json_replace "$.model" $request.model event_optional=true;
    }
  }
}
`), 0o600))

	_, err := ValidateProviderFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "event_optional requires event")
}

func TestValidateProviderFile_ResponseJSONEventOptionalSupportsAllEventScopedJSONOps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    response {
      json_set "$.model" $request.model event="message_start" event_optional=true;
      json_set_if_absent "$.role" "assistant" event="message_start" event_optional=true;
      json_del "$.usage" event="message_start" event_optional=true;
      json_rename "$.old" "$.new" event="message_start" event_optional=true;
    }
  }
}
`), 0o600))

	pf, err := ValidateProviderFile(path)
	require.NoError(t, err)
	require.Len(t, pf.Response.Defaults.JSONOps, 4)
	for _, op := range pf.Response.Defaults.JSONOps {
		require.Equal(t, "message_start", op.Event)
		require.True(t, op.EventOptional)
	}
}

func TestValidateProviderFile_SSEJSONDelIf_RejectsEmptyEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    response {
      sse_json_del_if "$.type" "" "$.usage";
    }
  }
}
`), 0o600))

	_, err := ValidateProviderFile(path)
	require.Error(t, err)
}

func TestValidateProviderFile_SSECollectWithRespMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
    response { resp_passthrough; }
  }

  match api = "chat.completions" stream = false {
    response {
      sse_collect openai_responses;
      resp_map openai_responses_to_openai_chat;
    }
  }
}
`), 0o600))

	pf, err := ValidateProviderFile(path)
	require.NoError(t, err)

	stream := false
	d, ok := pf.Response.Select(&dslmeta.Meta{API: "chat.completions", IsStream: stream})
	require.True(t, ok)
	require.Equal(t, "openai_responses", d.SSECollectMode)
	require.Equal(t, "resp_map", d.Op)
	require.Equal(t, "openai_responses_to_openai_chat", d.Mode)
}

func TestValidateProviderFile_SSECollectRejectsStreamTrue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
  }

  match api = "chat.completions" stream = true {
    response {
      sse_collect openai_responses;
    }
  }
}
`), 0o600))

	_, err := ValidateProviderFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stream = false")
}

func TestValidateProviderFile_SSECollectRejectsSSEParse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "t.conf")
	require.NoError(t, os.WriteFile(path, []byte(`
syntax "next-router/0.1";

provider "t" {
  defaults {
    upstream_config { base_url = "https://t.example.com"; }
  }

  match api = "chat.completions" stream = false {
    response {
      sse_collect openai_responses;
      sse_parse openai_responses_to_openai_chat_chunks;
    }
  }
}
`), 0o600))

	_, err := ValidateProviderFile(path)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be combined")
}

func TestTransformSSEEventDataJSON_ConditionalDelAndJSONOps(t *testing.T) {
	in := "" +
		"event: message\n" +
		"data: {\"type\":\"message_delta\",\"usage\":{\"input_tokens\":9},\"always\":1}\n\n" +
		"data: {\"type\":\"other\",\"usage\":{\"input_tokens\":1},\"always\":1}\n\n" +
		"data: [DONE]\n\n"

	rules := []SSEJSONDelIfRule{
		{CondPath: "$.type", Equals: "message_delta", DelPath: "$.usage"},
	}
	ops := []JSONOp{
		{Op: "json_del", Path: "$.always"},
	}

	var out bytes.Buffer
	err := TransformSSEEventDataJSON(bytes.NewBufferString(in), &out, &dslmeta.Meta{}, rules, ops)
	require.NoError(t, err)

	s := out.String()
	require.Contains(t, s, "event: message\n")
	require.Contains(t, s, "data: [DONE]\n\n")

	events := bytes.Split(out.Bytes(), []byte("\n\n"))
	var dataJSON [][]byte
	for _, ev := range events {
		lines := bytes.Split(ev, []byte("\n"))
		for _, l := range lines {
			l = bytes.TrimSpace(l)
			if bytes.HasPrefix(l, []byte("data:")) {
				p := bytes.TrimSpace(bytes.TrimPrefix(l, []byte("data:")))
				if len(p) > 0 && !bytes.Equal(p, []byte("[DONE]")) {
					dataJSON = append(dataJSON, p)
				}
			}
		}
	}
	require.Len(t, dataJSON, 2)

	var obj0 map[string]any
	require.NoError(t, json.Unmarshal(dataJSON[0], &obj0))
	require.Equal(t, "message_delta", obj0["type"])
	_, hasUsage0 := obj0["usage"]
	require.False(t, hasUsage0)
	_, hasAlways0 := obj0["always"]
	require.False(t, hasAlways0)

	var obj1 map[string]any
	require.NoError(t, json.Unmarshal(dataJSON[1], &obj1))
	require.Equal(t, "other", obj1["type"])
	_, hasUsage1 := obj1["usage"]
	require.True(t, hasUsage1)
	_, hasAlways1 := obj1["always"]
	require.False(t, hasAlways1)
}

func responseJSONPayloads(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var payloads []map[string]any
	for _, ev := range bytes.Split(raw, []byte("\n\n")) {
		for _, line := range bytes.Split(ev, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			body := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(body) == 0 || bytes.Equal(body, []byte("[DONE]")) {
				continue
			}
			var obj map[string]any
			require.NoError(t, json.Unmarshal(body, &obj))
			payloads = append(payloads, obj)
		}
	}
	return payloads
}
