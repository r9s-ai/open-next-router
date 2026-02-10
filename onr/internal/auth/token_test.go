package auth

import (
	"encoding/base64"
	"testing"
)

const testAccessKey = "ak-1"

func TestParseTokenKeyV1_UK64(t *testing.T) {
	ak64 := base64.RawURLEncoding.EncodeToString([]byte(testAccessKey))
	uk64 := base64.RawURLEncoding.EncodeToString([]byte("sk-upstream"))

	claims, accessKey, err := ParseTokenKeyV1("onr:v1?k64=" + ak64 + "&uk64=" + uk64 + "&p=openai&m=gpt-4o-mini")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if accessKey != testAccessKey {
		t.Fatalf("accessKey=%q", accessKey)
	}
	if claims.UpstreamKey != "sk-upstream" {
		t.Fatalf("upstream=%q", claims.UpstreamKey)
	}
	if claims.Mode != TokenModeBYOK {
		t.Fatalf("mode=%q", claims.Mode)
	}
}

func TestParseTokenKeyV1_InvalidUK64(t *testing.T) {
	_, _, err := ParseTokenKeyV1("onr:v1?k=" + testAccessKey + "&uk64=not-base64")
	if err == nil || err.Error() != "invalid uk64" {
		t.Fatalf("err=%v", err)
	}
}

func TestParseTokenKeyV1_UK64PreferredOverUK(t *testing.T) {
	uk64 := base64.RawURLEncoding.EncodeToString([]byte("sk-uk64"))
	claims, _, err := ParseTokenKeyV1("onr:v1?k=ak-1&uk=sk-plain&uk64=" + uk64)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if claims.UpstreamKey != "sk-uk64" {
		t.Fatalf("upstream=%q", claims.UpstreamKey)
	}
}
