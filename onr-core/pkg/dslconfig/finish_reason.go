package dslconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type FinishReasonExtractConfig struct {
	Mode             string
	FinishReasonPath string
}

type ProviderFinishReason struct {
	Defaults FinishReasonExtractConfig
	Matches  []MatchFinishReason
}

type MatchFinishReason struct {
	API    string
	Stream *bool

	Extract FinishReasonExtractConfig
}

func (p ProviderFinishReason) Select(meta *dslmeta.Meta) (FinishReasonExtractConfig, bool) {
	if meta == nil {
		return FinishReasonExtractConfig{}, false
	}
	// match overrides
	for _, m := range p.Matches {
		if strings.TrimSpace(m.API) != "" && strings.TrimSpace(m.API) != strings.TrimSpace(meta.API) {
			continue
		}
		if m.Stream != nil && *m.Stream != meta.IsStream {
			continue
		}
		cfg := mergeFinishReasonConfig(p.Defaults, m.Extract)
		if strings.TrimSpace(cfg.Mode) == "" && strings.TrimSpace(cfg.FinishReasonPath) == "" {
			return FinishReasonExtractConfig{}, false
		}
		return cfg, true
	}
	// defaults
	if strings.TrimSpace(p.Defaults.Mode) == "" && strings.TrimSpace(p.Defaults.FinishReasonPath) == "" {
		return FinishReasonExtractConfig{}, false
	}
	return p.Defaults, true
}

func mergeFinishReasonConfig(base, override FinishReasonExtractConfig) FinishReasonExtractConfig {
	out := base
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.FinishReasonPath) != "" {
		out.FinishReasonPath = override.FinishReasonPath
	}
	return out
}

// ExtractFinishReason extracts finish_reason from a JSON response (best-effort).
// Returns empty string when it cannot be extracted.
func ExtractFinishReason(meta *dslmeta.Meta, cfg FinishReasonExtractConfig, respBody []byte) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	path := strings.TrimSpace(cfg.FinishReasonPath)

	if mode == "" && path == "" {
		return "", nil
	}

	var obj any
	if err := json.Unmarshal(respBody, &obj); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return "", nil
	}

	if mode == "custom" {
		if strings.TrimSpace(path) == "" {
			return "", nil
		}
		return getStringByPath(root, path), nil
	}

	// Path override works for any non-custom mode as an escape hatch.
	if strings.TrimSpace(path) != "" {
		if v := getStringByPath(root, path); strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), nil
		}
	}

	switch mode {
	case "", "openai":
		return extractOpenAIFinishReason(root), nil
	case "anthropic":
		// Anthropic non-stream: stop_reason at top-level.
		// Anthropic stream events:
		// - message_start: {"message":{"stop_reason":null,...}}
		// - message_delta: {"delta":{"stop_reason":"end_turn",...}, "usage":{...}}
		return strings.TrimSpace(firstNonEmptyString(
			jsonutil.CoerceString(root["stop_reason"]),
			getStringByPath(root, "$.delta.stop_reason"),
			getStringByPath(root, "$.message.stop_reason"),
		)), nil
	case "gemini":
		return extractGeminiFinishReason(root), nil
	default:
		_ = meta
		return "", fmt.Errorf("unsupported finish_reason_extract mode %q", cfg.Mode)
	}
}

func extractOpenAIFinishReason(root map[string]any) string {
	// OpenAI-style: choices[*].finish_reason
	choices, ok := root["choices"].([]any)
	if !ok || len(choices) == 0 {
		return ""
	}
	for _, c := range choices {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if v := strings.TrimSpace(jsonutil.CoerceString(m["finish_reason"])); v != "" {
			return v
		}
	}
	return ""
}

func extractGeminiFinishReason(root map[string]any) string {
	// Gemini native: candidates[*].finishReason
	cands, ok := root["candidates"].([]any)
	if !ok || len(cands) == 0 {
		return ""
	}
	for _, c := range cands {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		if v := strings.TrimSpace(jsonutil.CoerceString(m["finishReason"])); v != "" {
			return v
		}
		// snake_case fallback
		if v := strings.TrimSpace(jsonutil.CoerceString(m["finish_reason"])); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmptyString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
