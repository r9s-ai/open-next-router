package dslconfig

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

func ApplyJSONOps(meta *dslmeta.Meta, in any, ops []JSONOp) (any, error) {
	if len(ops) == 0 {
		return in, nil
	}
	obj, ok := in.(map[string]any)
	if !ok {
		// Convert non-map inputs to a mutable JSON object representation.
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("marshal request json: %w", err)
		}
		var root any
		if err := json.Unmarshal(b, &root); err != nil {
			return nil, fmt.Errorf("unmarshal request json: %w", err)
		}
		obj, ok = root.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("request json root is not an object")
		}
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
		case jsonOpRename:
			opChanged, err := jsonRename(obj, op.FromPath, op.ToPath)
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
