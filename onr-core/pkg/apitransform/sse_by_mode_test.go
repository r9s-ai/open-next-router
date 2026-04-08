package apitransform

import (
	"bytes"
	"strings"
	"testing"
)

func TestSupportsSSETransformMode(t *testing.T) {
	if !SupportsSSETransformMode(" OpenAI_Responses_To_OpenAI_Chat_Chunks ") {
		t.Fatalf("expected normalized mode to be supported")
	}
	if SupportsSSETransformMode("unknown_mode") {
		t.Fatalf("expected unknown mode to be unsupported")
	}
}

func TestTransformSSEByMode_Unsupported(t *testing.T) {
	var out bytes.Buffer
	err := TransformSSEByMode("unknown_mode", strings.NewReader(""), &out)
	if err == nil {
		t.Fatalf("expected unsupported mode error")
	}
	if !strings.Contains(err.Error(), "unsupported sse_parse mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}
