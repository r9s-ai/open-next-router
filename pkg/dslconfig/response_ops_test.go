package dslconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/r9s-ai/open-next-router/pkg/dslmeta"
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
	require.Len(t, d.JSONOps, 4)
	require.Equal(t, "json_del", d.JSONOps[0].Op)
	require.Equal(t, "$.usage", d.JSONOps[0].Path)
	require.Equal(t, "json_set", d.JSONOps[1].Op)
	require.Equal(t, "$.foo", d.JSONOps[1].Path)
	require.Equal(t, "\"bar\"", d.JSONOps[1].ValueExpr)
	require.Equal(t, "json_set_if_absent", d.JSONOps[2].Op)
	require.Equal(t, "$.bar", d.JSONOps[2].Path)
	require.Equal(t, "\"baz\"", d.JSONOps[2].ValueExpr)
	require.Equal(t, "json_rename", d.JSONOps[3].Op)
	require.Equal(t, "$.a", d.JSONOps[3].FromPath)
	require.Equal(t, "$.b", d.JSONOps[3].ToPath)

	require.Len(t, d.SSEJSONDelIf, 1)
	require.Equal(t, "$.type", d.SSEJSONDelIf[0].CondPath)
	require.Equal(t, "message_delta", d.SSEJSONDelIf[0].Equals)
	require.Equal(t, "$.usage", d.SSEJSONDelIf[0].DelPath)
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
