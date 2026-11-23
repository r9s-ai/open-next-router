package dslconfig

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/stretchr/testify/require"
)

func TestSSEJSONOpsFastPath(t *testing.T) {
	meta := &dslmeta.Meta{OriginModelName: "client"}
	replace := JSONOp{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model", Event: "message_start", MaxCount: 1}
	tests := []struct {
		name  string
		input string
		rules []SSEJSONDelIfRule
		ops   []JSONOp
		want  string
	}{
		{
			name:  "event mismatch",
			input: "event: message_delta\ndata: { \"model\" : \"upstream\" }\n\n",
			ops:   []JSONOp{replace},
			want:  "event: message_delta\ndata: { \"model\" : \"upstream\" }\n\n",
		},
		{
			name:  "missing event",
			input: "data: { \"model\" : \"upstream\" }\n\n",
			ops:   []JSONOp{replace},
			want:  "data: { \"model\" : \"upstream\" }\n\n",
		},
		{
			name:  "invalid payload on skipped event",
			input: "event: message_delta\ndata: {broken}\n\n",
			ops:   []JSONOp{replace},
			want:  "event: message_delta\ndata: {broken}\n\n",
		},
		{
			name:  "optional missing event",
			input: "data: { \"model\" : \"upstream\" }\n\n",
			ops:   []JSONOp{{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model", Event: "message_start", EventOptional: true}},
			want:  "data: {\"model\":\"client\"}\n\n",
		},
		{
			name:  "another op applies",
			input: "event: message_delta\ndata: { \"model\" : \"upstream\" }\n\n",
			ops:   []JSONOp{replace, {Op: "json_set", Path: "$.flag", ValueExpr: "true", Event: "message_delta"}},
			want:  "event: message_delta\ndata: {\"flag\":true,\"model\":\"upstream\"}\n\n",
		},
		{
			name:  "deletion rule applies",
			input: "event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":1}\n\n",
			rules: []SSEJSONDelIfRule{{CondPath: "$.type", Equals: "message_delta", DelPath: "$.usage"}},
			ops:   []JSONOp{replace},
			want:  "event: message_delta\ndata: {\"type\":\"message_delta\"}\n\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bytes.Buffer
			require.NoError(t, TransformSSEEventDataJSON(strings.NewReader(tt.input), &got, meta, tt.rules, tt.ops))
			require.Equal(t, tt.want, got.String())
		})
	}
}

func TestSSEJSONOpsFastPathCountsOnlyChanges(t *testing.T) {
	input := "event: message_start\ndata: { \"other\": 1 }\n\n" +
		"event: message_delta\ndata: { \"model\": \"delta\" }\n\n" +
		"event: message_start\ndata: {\"model\":\"upstream\"}\n\n" +
		"event: message_start\ndata: { \"model\" : \"later\" }\n\n"
	want := "event: message_start\ndata: {\"other\":1}\n\n" +
		"event: message_delta\ndata: { \"model\": \"delta\" }\n\n" +
		"event: message_start\ndata: {\"model\":\"client\"}\n\n" +
		"event: message_start\ndata: { \"model\" : \"later\" }\n\n"
	ops := []JSONOp{{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model", Event: "message_start", MaxCount: 1}}
	var got bytes.Buffer
	require.NoError(t, TransformSSEEventDataJSON(strings.NewReader(input), &got, &dslmeta.Meta{OriginModelName: "client"}, nil, ops))
	require.Equal(t, want, got.String())
}

func TestSSEJSONOpsFastPathReaderWriterAgree(t *testing.T) {
	input := "event: message_start\ndata: {\ndata: \"model\": \"upstream\"}\n\n" +
		"event: message_delta\ndata: {\ndata: \"delta\": \"text\"}\n\n" +
		"data: [DONE]"
	ops := []JSONOp{{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model", Event: "message_start", MaxCount: 1}}
	meta := &dslmeta.Meta{OriginModelName: "client"}

	var fromReader bytes.Buffer
	require.NoError(t, TransformSSEEventDataJSON(strings.NewReader(input), &fromReader, meta, nil, ops))
	require.Contains(t, fromReader.String(), "data: {\"model\":\"client\"}\n\n")
	require.Contains(t, fromReader.String(), "data: {\"delta\":\"text\"}\n\n")
	require.True(t, strings.HasSuffix(fromReader.String(), "data: [DONE]\n\n"))

	var fromWriter bytes.Buffer
	w := NewSSEEventDataJSONWriter(&fromWriter, meta, nil, ops)
	for _, part := range []string{input[:17], input[17:55], input[55:]} {
		_, err := io.WriteString(w, part)
		require.NoError(t, err)
	}
	require.NoError(t, w.Finish())
	require.Equal(t, fromReader.String(), fromWriter.String())
}

func BenchmarkTransformSSEEventDataJSON_FirstEventOnly(b *testing.B) {
	var input strings.Builder
	input.WriteString("event: message_start\ndata: {\"model\":\"upstream\"}\n\n")
	for range 256 {
		input.WriteString("event: message_delta\ndata: {\"delta\":\"text\"}\n\n")
	}
	stream := input.String()
	ops := []JSONOp{{Op: "json_replace", Path: "$.model", ValueExpr: "$request.model", Event: "message_start", MaxCount: 1}}
	meta := &dslmeta.Meta{OriginModelName: "client"}
	var out bytes.Buffer
	out.Grow(len(stream))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		out.Reset()
		if err := TransformSSEEventDataJSON(strings.NewReader(stream), &out, meta, nil, ops); err != nil {
			b.Fatal(err)
		}
	}
}
