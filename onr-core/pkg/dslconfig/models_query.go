package dslconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

const (
	modelsModeOpenAI = "openai"
	modelsModeGemini = "gemini"
	modelsModeCustom = "custom"
)

type ModelsQueryConfig struct {
	Mode string `json:"mode,omitempty"`

	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`

	IDPaths      []string `json:"id_paths,omitempty"`
	IDRegex      string   `json:"id_regex,omitempty"`
	IDAllowRegex string   `json:"id_allow_regex,omitempty"`

	Headers []HeaderOp `json:"headers,omitempty"`
}

type ProviderModels struct {
	Defaults ModelsQueryConfig
}

func (p ProviderModels) Select(_ *dslmeta.Meta) (ModelsQueryConfig, bool) {
	cfg := normalizeModelsQueryConfig(p.Defaults)
	if strings.TrimSpace(cfg.Mode) == "" {
		return ModelsQueryConfig{}, false
	}
	return cfg, true
}

func normalizeModelsQueryConfig(in ModelsQueryConfig) ModelsQueryConfig {
	out := in
	mode := strings.ToLower(strings.TrimSpace(in.Mode))
	out.Mode = mode

	if strings.TrimSpace(out.Method) == "" {
		out.Method = "GET"
	}
	if mode == modelsModeOpenAI {
		if strings.TrimSpace(out.Path) == "" {
			out.Path = "/v1/models"
		}
		if len(out.IDPaths) == 0 {
			out.IDPaths = []string{"$.data[*].id"}
		}
	}
	if mode == modelsModeGemini {
		if strings.TrimSpace(out.Path) == "" {
			out.Path = "/v1beta/models"
		}
		if len(out.IDPaths) == 0 {
			out.IDPaths = []string{"$.models[*].name"}
		}
		if strings.TrimSpace(out.IDRegex) == "" {
			out.IDRegex = "^models/(.+)$"
		}
	}
	return out
}

func mergeModelsQueryConfig(base ModelsQueryConfig, override ModelsQueryConfig) ModelsQueryConfig {
	out := base
	if strings.TrimSpace(override.Mode) != "" {
		out.Mode = override.Mode
	}
	if strings.TrimSpace(override.Method) != "" {
		out.Method = override.Method
	}
	if strings.TrimSpace(override.Path) != "" {
		out.Path = override.Path
	}
	if len(override.IDPaths) > 0 {
		out.IDPaths = append([]string(nil), override.IDPaths...)
	}
	if strings.TrimSpace(override.IDRegex) != "" {
		out.IDRegex = override.IDRegex
	}
	if strings.TrimSpace(override.IDAllowRegex) != "" {
		out.IDAllowRegex = override.IDAllowRegex
	}
	if len(override.Headers) > 0 {
		out.Headers = append([]HeaderOp(nil), override.Headers...)
	}
	return normalizeModelsQueryConfig(out)
}

func builtinModelsPresetName(mode string) string {
	switch normalizeUsageMode(mode) {
	case modelsModeOpenAI:
		return modelsModeOpenAI
	case modelsModeGemini:
		return modelsModeGemini
	default:
		return ""
	}
}

func ExtractModelIDs(cfg ModelsQueryConfig, respBody []byte) ([]string, error) {
	normalized := normalizeModelsQueryConfig(cfg)
	if len(normalized.IDPaths) == 0 {
		return nil, nil
	}

	var data any
	if err := json.Unmarshal(respBody, &data); err != nil {
		return nil, fmt.Errorf("parse response json: %w", err)
	}

	var (
		rewriteRE *regexp.Regexp
		allowRE   *regexp.Regexp
		err       error
	)
	if strings.TrimSpace(normalized.IDRegex) != "" {
		rewriteRE, err = regexp.Compile(strings.TrimSpace(normalized.IDRegex))
		if err != nil {
			return nil, fmt.Errorf("invalid id_regex: %w", err)
		}
	}
	if strings.TrimSpace(normalized.IDAllowRegex) != "" {
		allowRE, err = regexp.Compile(strings.TrimSpace(normalized.IDAllowRegex))
		if err != nil {
			return nil, fmt.Errorf("invalid id_allow_regex: %w", err)
		}
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, 32)
	for _, path := range normalized.IDPaths {
		values := extractStringsByPath(data, path)
		for _, raw := range values {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			if rewriteRE != nil {
				matches := rewriteRE.FindStringSubmatch(id)
				if len(matches) == 0 {
					continue
				}
				if len(matches) >= 2 {
					id = strings.TrimSpace(matches[1])
				} else {
					id = strings.TrimSpace(matches[0])
				}
				if id == "" {
					continue
				}
			}
			if allowRE != nil && !allowRE.MatchString(id) {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out, nil
}

func extractStringsByPath(root any, path string) []string {
	out := make([]string, 0)
	if !jsonutil.VisitValuesByPath(root, path, func(v any) {
		s := strings.TrimSpace(jsonutil.CoerceString(v))
		if s != "" {
			out = append(out, s)
		}
	}) {
		return nil
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
