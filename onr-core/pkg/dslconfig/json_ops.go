package dslconfig

import (
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

// ApplyJSONOps applies object-path JSON mutations to a top-level object root and returns it.
// in must already be normalized to map[string]any by the caller.
func ApplyJSONOps(meta *dslmeta.Meta, in map[string]any, ops []JSONOp) (map[string]any, error) {
	obj := in
	if obj == nil {
		return nil, fmt.Errorf("request json root is nil")
	}

	for _, op := range ops {
		switch op.Op {
		case jsonOpSet:
			val := evalJSONValueExpr(meta, op.ValueExpr)
			opChanged, err := jsonSet(obj, op.Path, val)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpSetIfAbsent:
			exists, err := jsonPathExists(obj, op.Path)
			if err != nil {
				return nil, err
			}
			if exists {
				continue
			}
			val := evalJSONValueExpr(meta, op.ValueExpr)
			opChanged, err := jsonSet(obj, op.Path, val)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpDel:
			opChanged, err := jsonDel(obj, op.Path)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpDelIfMissing:
			exists, err := jsonPathExists(obj, op.FromPath)
			if err != nil {
				return nil, err
			}
			if exists {
				continue
			}
			opChanged, err := jsonDel(obj, op.Path)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpRename:
			opChanged, err := jsonRename(obj, op.FromPath, op.ToPath)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpWrapInputText:
			opChanged, err := jsonWrapInputText(obj, op.Path)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpSetHeaderVals:
			// Filtering is intentionally handled by a following json_filter_values op.
			// The parser rejects extra value patterns on json_set_header_values so
			// config authors do not assume this op filters values by itself.
			vals := headerValuesForJSON(meta, op.HeaderName, op.Separator)
			if len(vals) == 0 {
				continue
			}
			opChanged, err := jsonSet(obj, op.Path, vals)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpFilterValues:
			opChanged, err := jsonFilterValues(obj, op.Path, op.Patterns)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		case jsonOpDelWithCond:
			opChanged, err := jsonDelWithCondition(obj, op.Path, op.FieldName, op.Patterns)
			if err != nil {
				return nil, err
			}
			_ = opChanged
		default:
			return nil, fmt.Errorf("unsupported json op %q", op.Op)
		}
	}
	return obj, nil
}

// headerValuesForJSON reads header values from the original downstream user request.
// It intentionally does not read headers prepared for the upstream request, so
// JSON body fields can still be populated from headers that request rules delete
// before forwarding upstream.
func headerValuesForJSON(meta *dslmeta.Meta, headerName string, separator string) []string {
	name := strings.TrimSpace(headerName)
	if meta == nil || meta.RequestHeaders == nil || name == "" {
		return nil
	}
	return splitHeaderValues(meta.RequestHeaders.Values(name), separator)
}

func lowerPatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		out = append(out, strings.ToLower(strings.TrimSpace(pattern)))
	}
	return out
}

func jsonPathExists(root map[string]any, path string) (bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return false, nil
		}
		m, ok := next.(map[string]any)
		if !ok {
			return false, nil
		}
		cur = m
	}
	_, ok := cur[parts[len(parts)-1]]
	return ok, nil
}

func evalJSONValueExpr(meta *dslmeta.Meta, expr string) any {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return ""
	}
	switch raw {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if strings.HasPrefix(raw, "\"") && strings.HasSuffix(raw, "\"") {
		return unquoteString(raw)
	}
	if i, err := strconv.Atoi(raw); err == nil {
		return i
	}
	// fall back to string expression evaluation
	return evalStringExpr(raw, meta)
}

func jsonSet(root map[string]any, path string, val any) (bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	cur := root
	changed := false
	for i := 0; i < len(parts)-1; i++ {
		k := parts[i]
		next, ok := cur[k]
		if ok {
			if m, ok := next.(map[string]any); ok {
				cur = m
				continue
			}
		}
		m := map[string]any{}
		cur[k] = m
		cur = m
		changed = true
	}
	last := parts[len(parts)-1]
	if old, ok := cur[last]; ok && reflect.DeepEqual(old, val) {
		return changed, nil
	}
	cur[last] = val
	return true, nil
}

func jsonDel(root map[string]any, path string) (bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		k := parts[i]
		next, ok := cur[k]
		if !ok {
			return false, nil
		}
		m, ok := next.(map[string]any)
		if !ok {
			return false, nil
		}
		cur = m
	}
	last := parts[len(parts)-1]
	if _, ok := cur[last]; !ok {
		return false, nil
	}
	delete(cur, last)
	return true, nil
}

func jsonRename(root map[string]any, fromPath string, toPath string) (bool, error) {
	fromParts, err := parseObjectPath(fromPath)
	if err != nil {
		return false, err
	}
	toParts, err := parseObjectPath(toPath)
	if err != nil {
		return false, err
	}
	if len(fromParts) == 0 || len(toParts) == 0 {
		return false, nil
	}

	var val any
	// get + delete
	{
		cur := root
		for i := 0; i < len(fromParts)-1; i++ {
			next, ok := cur[fromParts[i]]
			if !ok {
				return false, nil
			}
			m, ok := next.(map[string]any)
			if !ok {
				return false, nil
			}
			cur = m
		}
		last := fromParts[len(fromParts)-1]
		v, ok := cur[last]
		if !ok {
			return false, nil
		}
		val = v
		delete(cur, last)
	}

	// set
	{
		cur := root
		changed := true
		for i := 0; i < len(toParts)-1; i++ {
			k := toParts[i]
			next, ok := cur[k]
			if ok {
				if m, ok := next.(map[string]any); ok {
					cur = m
					continue
				}
			}
			m := map[string]any{}
			cur[k] = m
			cur = m
			changed = true
		}
		last := toParts[len(toParts)-1]
		if old, ok := cur[last]; ok && reflect.DeepEqual(old, val) {
			return changed, nil
		}
		cur[last] = val
	}
	return true, nil
}

func jsonWrapInputText(root map[string]any, path string) (bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return false, nil
		}
		m, ok := next.(map[string]any)
		if !ok {
			return false, nil
		}
		cur = m
	}
	last := parts[len(parts)-1]
	val, ok := cur[last]
	if !ok {
		return false, nil
	}
	if text, ok := val.(string); ok {
		cur[last] = []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{
						"type": "input_text",
						"text": text,
					},
				},
			},
		}
		return true, nil
	}
	if val != nil && reflect.TypeOf(val).Kind() == reflect.Slice {
		return false, nil
	}
	return false, fmt.Errorf("json_wrap_input_text %s expects string or array, got %T", path, val)
}

func jsonFilterValues(root map[string]any, path string, patterns []string) (bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return false, err
	}
	if len(parts) == 0 {
		return false, nil
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return false, nil
		}
		m, ok := next.(map[string]any)
		if !ok {
			return false, nil
		}
		cur = m
	}
	last := parts[len(parts)-1]
	val, ok := cur[last]
	if !ok {
		return false, nil
	}
	values, ok := stringSliceValue(val)
	if !ok {
		return false, fmt.Errorf("json_filter_values %s expects string array, got %T", path, val)
	}
	loweredPatterns := lowerPatterns(patterns)
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if matchesAnyHeaderValuePattern(strings.ToLower(value), loweredPatterns) {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		delete(cur, last)
		return true, nil
	}
	if reflect.DeepEqual(values, filtered) {
		return false, nil
	}
	cur[last] = filtered
	return true, nil
}

func stringSliceValue(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return typed, true
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func jsonDelWithCondition(root map[string]any, path string, fieldName string, patterns []string) (bool, error) {
	parent, key, ok, err := jsonParentAndKey(root, path)
	if err != nil || !ok {
		return false, err
	}
	val, ok := parent[key]
	if !ok {
		return false, nil
	}
	switch typed := val.(type) {
	case []any:
		filtered := make([]any, 0, len(typed))
		for _, item := range typed {
			obj, ok := item.(map[string]any)
			if ok && jsonObjectFieldMatches(obj, fieldName, patterns) {
				continue
			}
			filtered = append(filtered, item)
		}
		if len(filtered) == len(typed) {
			return false, nil
		}
		if len(filtered) == 0 {
			delete(parent, key)
			return true, nil
		}
		parent[key] = filtered
		return true, nil
	case map[string]any:
		if !jsonObjectFieldMatches(typed, fieldName, patterns) {
			return false, nil
		}
		delete(parent, key)
		return true, nil
	default:
		slog.Debug("json_del_with_condition ignored non-object value", "path", path, "type", fmt.Sprintf("%T", val))
		return false, nil
	}
}

func jsonObjectFieldMatches(obj map[string]any, fieldName string, patterns []string) bool {
	fieldValue, ok := obj[strings.TrimSpace(fieldName)].(string)
	if !ok {
		return false
	}
	return matchesAnyHeaderValuePattern(strings.ToLower(fieldValue), lowerPatterns(patterns))
}

func jsonParentAndKey(root map[string]any, path string) (map[string]any, string, bool, error) {
	parts, err := parseObjectPath(path)
	if err != nil {
		return nil, "", false, err
	}
	if len(parts) == 0 {
		return nil, "", false, nil
	}
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return nil, "", false, nil
		}
		m, ok := next.(map[string]any)
		if !ok {
			return nil, "", false, nil
		}
		cur = m
	}
	return cur, parts[len(parts)-1], true, nil
}

func parseObjectPath(path string) ([]string, error) {
	p := strings.TrimSpace(path)
	if !strings.HasPrefix(p, "$.") {
		return nil, fmt.Errorf("json path must start with '$.': %q", path)
	}
	rest := strings.TrimPrefix(p, "$.")
	if strings.Contains(rest, "[") || strings.Contains(rest, "]") {
		return nil, fmt.Errorf("json path does not support array indexes in v0.1: %q", path)
	}
	parts := strings.Split(rest, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		k := strings.TrimSpace(part)
		if k == "" {
			return nil, fmt.Errorf("invalid json path: %q", path)
		}
		out = append(out, k)
	}
	return out, nil
}
