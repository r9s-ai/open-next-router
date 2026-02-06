package auth

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	//nolint:gosec // context key identifier, not credential material
	ctxTokenProvider = "onr.token_provider"
	//nolint:gosec // context key identifier, not credential material
	ctxTokenModelOverride = "onr.token_model_override"
	//nolint:gosec // context key identifier, not credential material
	ctxTokenUpstreamKey = "onr.token_upstream_key"
	//nolint:gosec // context key identifier, not credential material
	ctxTokenMode = "onr.token_mode"
)

// TokenMode represents how upstream key is sourced.
type TokenMode string

const (
	TokenModeONR  TokenMode = "onr"
	TokenModeBYOK TokenMode = "byok"
)

type TokenClaims struct {
	Provider      string
	ModelOverride string
	UpstreamKey   string
	Mode          TokenMode
	ExpUnix       int64
}

func (c TokenClaims) Expired(nowUnix int64) bool {
	if c.ExpUnix <= 0 {
		return false
	}
	return nowUnix >= c.ExpUnix
}

func IsTokenKey(raw string) bool {
	return strings.HasPrefix(strings.TrimSpace(raw), "onr:v1?")
}

// ParseTokenKeyV1 parses an onr token key (no signature).
//
//	onr:v1?k=<access_key>&...
//	onr:v1?k64=<base64url(access_key)>&...
//
// Supported query params:
// - k / k64: access key (required)
// - p: provider (optional)
// - m: model_override (optional)
// - uk: upstream key for BYOK (optional; implies mode=byok)
// - exp: unix seconds expiry (optional)
func ParseTokenKeyV1(raw string, now time.Time) (*TokenClaims, string, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "onr:v1?") {
		return nil, "", fmt.Errorf("not an onr:v1 token key")
	}
	qraw := strings.TrimPrefix(s, "onr:v1?")
	vals, err := url.ParseQuery(qraw)
	if err != nil {
		return nil, "", fmt.Errorf("invalid token query: %w", err)
	}

	accessKey := ""
	if k64 := strings.TrimSpace(vals.Get("k64")); k64 != "" {
		b, err := base64.RawURLEncoding.DecodeString(k64)
		if err != nil {
			return nil, "", fmt.Errorf("invalid k64")
		}
		accessKey = strings.TrimSpace(string(b))
	} else if k := strings.TrimSpace(vals.Get("k")); k != "" {
		accessKey = k
	} else {
		return nil, "", fmt.Errorf("missing k or k64")
	}

	claims := &TokenClaims{
		Provider:      strings.ToLower(strings.TrimSpace(vals.Get("p"))),
		ModelOverride: strings.TrimSpace(vals.Get("m")),
		UpstreamKey:   strings.TrimSpace(vals.Get("uk")),
		Mode:          TokenModeONR,
	}
	if claims.UpstreamKey != "" {
		claims.Mode = TokenModeBYOK
	}
	if expStr := strings.TrimSpace(vals.Get("exp")); expStr != "" {
		n, err := strconv.ParseInt(expStr, 10, 64)
		if err != nil || n <= 0 {
			return nil, "", fmt.Errorf("invalid exp")
		}
		claims.ExpUnix = n
		if claims.Expired(now.Unix()) {
			return nil, "", fmt.Errorf("token expired")
		}
	}
	return claims, accessKey, nil
}
