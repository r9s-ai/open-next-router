package dslconfig

import (
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

type usageRootConfig struct {
	Path          string
	Event         string
	EventOptional bool
	ExcludeFields []string
	// Source controls which request body the path is evaluated against.
	// "request" reads from the upstream-sent request root; empty or "response"
	// reads from the upstream response root (the default).
	Source string
}

// UsageRootRule describes one usage_root extraction rule in DSL/runtime.
type UsageRootRule = usageRootConfig

// extractUsageRootWithEvent builds the merged usage root object from matching
// usage_root rules. reqRoot is the upstream request body (used when
// source=request); respRoot is the upstream response body (default source).
func extractUsageRootWithEvent(event string, reqRoot, respRoot map[string]any, roots []usageRootConfig) map[string]any {
	if len(roots) == 0 {
		return nil
	}
	merged := map[string]any{}
	for _, rootCfg := range roots {
		if !matchesUsageEvent(event, rootCfg.Event, rootCfg.EventOptional) {
			continue
		}
		src := respRoot
		if strings.EqualFold(strings.TrimSpace(rootCfg.Source), "request") {
			src = reqRoot
		}
		if len(src) == 0 {
			continue
		}
		values, ok := jsonutil.GetValuesByPath(src, rootCfg.Path)
		if !ok {
			continue
		}
		for _, value := range values {
			obj, ok := value.(map[string]any)
			if !ok {
				// Scalar path (e.g. path="$.duration"): contribute {last_segment: value}
				// so individual root fields can be captured without pulling the whole root.
				if key := lastScalarPathKey(rootCfg.Path); key != "" {
					mergeUsageRootPreferNonZero(merged, map[string]any{key: value})
				}
				continue
			}
			obj = cloneUsageRootValueWithExcludedFields(obj, rootCfg.ExcludeFields)
			mergeUsageRootPreferNonZero(merged, obj)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// lastScalarPathKey returns the bare field name from the last segment of a
// $.a.b.c style path, for use when the resolved value is a scalar rather than
// an object.  Returns "" for the root path "$" or for paths with brackets.
func lastScalarPathKey(path string) string {
	trimmed := strings.TrimSpace(path)
	if !strings.HasPrefix(trimmed, "$.") {
		return ""
	}
	after := trimmed[2:]
	if idx := strings.LastIndex(after, "."); idx >= 0 {
		after = after[idx+1:]
	}
	if idx := strings.IndexByte(after, '['); idx >= 0 {
		after = after[:idx]
	}
	return strings.TrimSpace(after)
}

func cloneUsageRootValueWithExcludedFields(obj map[string]any, fields []string) map[string]any {
	if len(obj) == 0 {
		return nil
	}
	out := cloneUsageRootValue(obj)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		delete(out, field)
	}
	return out
}

func mergeUsageRootPreferNonZero(dst, src map[string]any) {
	for key, next := range src {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if current, ok := dst[key]; ok {
			currentMap, currentIsMap := current.(map[string]any)
			nextMap, nextIsMap := next.(map[string]any)
			if currentIsMap && nextIsMap {
				mergeUsageRootPreferNonZero(currentMap, nextMap)
				continue
			}
			if usageRootValueShouldReplace(current, next) {
				dst[key] = next
			}
			continue
		}
		dst[key] = next
	}
}

func usageRootValueShouldReplace(current, next any) bool {
	return usageRootValueIsEmpty(current) && !usageRootValueIsEmpty(next)
}

func usageRootValueIsEmpty(v any) bool {
	if v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t) == ""
	case bool:
		return !t
	case map[string]any:
		return len(t) == 0
	case []any:
		return len(t) == 0
	default:
		if n, ok := usageFlatFieldInt(v); ok {
			return n == 0
		}
		return false
	}
}

func mergeUsageRootLatestNonZero(dst, src map[string]any) {
	for key, next := range src {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if current, ok := dst[key]; ok {
			currentMap, currentIsMap := current.(map[string]any)
			nextMap, nextIsMap := next.(map[string]any)
			if currentIsMap && nextIsMap {
				mergeUsageRootLatestNonZero(currentMap, nextMap)
				continue
			}
			if !usageRootValueIsEmpty(next) || usageRootValueIsEmpty(current) {
				dst[key] = next
			}
			continue
		}
		dst[key] = next
	}
}

func matchesUsageEvent(current, expected string, optional bool) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	current = strings.TrimSpace(current)
	if current == "" && optional {
		return true
	}
	for _, part := range strings.Split(expected, "|") {
		if strings.EqualFold(strings.TrimSpace(part), current) {
			return true
		}
	}
	return false
}

func cloneUsageRootConfigs(roots []usageRootConfig) []usageRootConfig {
	if len(roots) == 0 {
		return nil
	}
	out := make([]usageRootConfig, len(roots))
	for i, root := range roots {
		out[i] = root
		if len(root.ExcludeFields) > 0 {
			out[i].ExcludeFields = append([]string(nil), root.ExcludeFields...)
		}
	}
	return out
}

func cloneUsageRootValue(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		if child, ok := v.(map[string]any); ok {
			out[k] = cloneUsageRootValue(child)
			continue
		}
		out[k] = v
	}
	return out
}
