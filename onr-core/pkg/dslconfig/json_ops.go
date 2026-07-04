package dslconfig

import (
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// ApplyJSONOps applies object-path JSON mutations to a top-level object root and returns it.
// in must already be normalized to map[string]any by the caller.
func ApplyJSONOps(meta *dslmeta.Meta, in map[string]any, ops []JSONOp) (map[string]any, error) {
	obj := in
	if obj == nil {
		return nil, fmt.Errorf("request json root is nil")
	}

	counts := make([]int, len(ops))
	for i, op := range ops {
		if shouldSkipJSONOp("", op, counts, i) {
			continue
		}
		changed := false
		switch op.Op {
		case jsonOpSet:
			val := evalJSONValueExpr(meta, op.ValueExpr)
			opChanged, err := jsonSet(obj, op.Path, val)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		case jsonOpReplace:
			val := evalJSONValueExpr(meta, op.ValueExpr)
			opChanged, err := jsonReplace(obj, op.Path, val)
			if err != nil {
				return nil, err
			}
			changed = opChanged
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
			changed = opChanged
		case jsonOpDel:
			opChanged, err := jsonDel(obj, op.Path)
			if err != nil {
				return nil, err
			}
			changed = opChanged
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
			changed = opChanged
		case jsonOpRename:
			opChanged, err := jsonRename(obj, op.FromPath, op.ToPath)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		case jsonOpWrapInputText:
			opChanged, err := jsonWrapInputText(obj, op.Path)
			if err != nil {
				return nil, err
			}
			changed = opChanged
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
			changed = opChanged
		case jsonOpFilterValues:
			opChanged, err := jsonFilterValues(obj, op.Path, op.Patterns)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		case jsonOpDelWithCond:
			opChanged, err := jsonDelWithCondition(obj, op.Path, op.FieldName, op.Patterns)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		case jsonOpMapValue:
			val := evalJSONValueExpr(meta, op.ValueExpr)
			opChanged, err := jsonMapValue(obj, op.Path, op.MatchValue, val)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		case jsonOpScale:
			opChanged, err := jsonScale(obj, op.Path, op.ScaleRange)
			if err != nil {
				return nil, err
			}
			changed = opChanged
		default:
			return nil, fmt.Errorf("unsupported json op %q", op.Op)
		}
		recordJSONOpChange(changed, op, counts, i)
	}
	return obj, nil
}

func shouldSkipJSONOp(event string, op JSONOp, counts []int, idx int) bool {
	if strings.TrimSpace(op.Event) != "" && !matchesUsageEvent(event, op.Event, op.EventOptional) {
		return true
	}
	return op.MaxCount > 0 && counts != nil && idx >= 0 && idx < len(counts) && counts[idx] >= op.MaxCount
}

func recordJSONOpChange(changed bool, op JSONOp, counts []int, idx int) {
	if !changed || op.MaxCount <= 0 || counts == nil || idx < 0 || idx >= len(counts) {
		return
	}
	counts[idx]++
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
	if f, ok := parseFloatLiteral(raw); ok {
		return f
	}
	// fall back to string expression evaluation
	return evalStringExpr(raw, meta)
}

// parseFloatLiteral parses a plain decimal float literal like "1.0" or "-0.5".
// It intentionally rejects Inf/NaN and exotic forms so words like "inf" stay strings.
func parseFloatLiteral(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	c := raw[0]
	if c != '-' && c != '+' && c != '.' && (c < '0' || c > '9') {
		return 0, false
	}
	if !strings.ContainsAny(raw, ".eE") {
		return 0, false
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

// jsonMapValue replaces the string value at path with val only when it equals
// matchValue. Missing paths and non-matching/non-string values are left unchanged,
// so unmapped values pass through (same semantics as model_map fallthrough).
func jsonMapValue(root map[string]any, path string, matchValue string, val any) (bool, error) {
	parent, key, ok, err := jsonParentAndKey(root, path)
	if err != nil || !ok {
		return false, err
	}
	cur, ok := parent[key]
	if !ok {
		return false, nil
	}
	s, ok := cur.(string)
	if !ok || s != matchValue {
		return false, nil
	}
	if reflect.DeepEqual(cur, val) {
		return false, nil
	}
	parent[key] = val
	return true, nil
}

// jsonScale linearly maps the numeric value at path from [InMin, InMax] onto
// [OutMin, OutMax] with clamping. Missing paths and non-numeric values are left
// unchanged so optional fields pass through.
func jsonScale(root map[string]any, path string, r *JSONScaleRange) (bool, error) {
	if r == nil {
		// The parser always attaches ScaleRange to json_scale ops; a nil range here
		// means the op was constructed programmatically in an invalid way.
		return false, fmt.Errorf("json_scale %s missing scale range", path)
	}
	parent, key, ok, err := jsonParentAndKey(root, path)
	if err != nil || !ok {
		return false, err
	}
	cur, ok := parent[key]
	if !ok {
		return false, nil
	}
	v, ok := jsonutil.CoerceFloatOK(cur)
	if !ok {
		return false, nil
	}
	if v < r.InMin {
		v = r.InMin
	}
	if v > r.InMax {
		v = r.InMax
	}
	out := r.OutMin + (v-r.InMin)*(r.OutMax-r.OutMin)/(r.InMax-r.InMin)
	if reflect.DeepEqual(cur, out) {
		return false, nil
	}
	parent[key] = out
	return true, nil
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

func jsonReplace(root map[string]any, path string, val any) (bool, error) {
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
		if !ok || m == nil {
			return false, nil
		}
		cur = m
	}
	last := parts[len(parts)-1]
	old, ok := cur[last]
	if !ok {
		return false, nil
	}
	if reflect.DeepEqual(old, val) {
		return false, nil
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
