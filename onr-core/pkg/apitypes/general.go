package apitypes

import "fmt"

type FromMapper interface {
	FromMap(map[string]any) error
}

type ToMapper interface {
	ToMap() (map[string]any, error)
}

func toInt64(v any) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case float64:
		return int64(n), nil
	case float32:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("expected numeric value, got %T", v)
	}
}

func toFloat64(v any) (float64, error) {
	switch n := v.(type) {
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("expected numeric value, got %T", v)
	}
}

func mapValue(m map[string]any, key string) (any, bool) {
	v, ok := m[key]
	return v, ok
}

func stringValue(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be string, got %T", key, v)
	}
	return s, nil
}

func intValue(m map[string]any, key string) (int, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, nil
	}
	n, err := toInt64(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return int(n), nil
}

func floatValue(m map[string]any, key string) (float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return 0, nil
	}
	n, err := toFloat64(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func boolValue(m map[string]any, key string) (bool, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return false, nil
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be bool, got %T", key, v)
	}
	return b, nil
}

func intPtrValue(m map[string]any, key string) (*int, error) {
	if _, ok := m[key]; !ok || m[key] == nil {
		return nil, nil
	}
	v, err := intValue(m, key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func floatPtrValue(m map[string]any, key string) (*float64, error) {
	if _, ok := m[key]; !ok || m[key] == nil {
		return nil, nil
	}
	v, err := floatValue(m, key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func boolPtrValue(m map[string]any, key string) (*bool, error) {
	if _, ok := m[key]; !ok || m[key] == nil {
		return nil, nil
	}
	v, err := boolValue(m, key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func stringPtrValue(m map[string]any, key string) (*string, error) {
	if _, ok := m[key]; !ok || m[key] == nil {
		return nil, nil
	}
	v, err := stringValue(m, key)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func stringSliceValue(m map[string]any, key string) ([]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	switch items := v.(type) {
	case []string:
		out := make([]string, len(items))
		copy(out, items)
		return out, nil
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s item must be string, got %T", key, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("%s must be []string or []any, got %T", key, v)
	}
}

func intSliceValue(m map[string]any, key string) ([]int, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be []any, got %T", key, v)
	}
	out := make([]int, 0, len(items))
	for _, item := range items {
		n, err := toInt64(item)
		if err != nil {
			return nil, fmt.Errorf("%s item: %w", key, err)
		}
		out = append(out, int(n))
	}
	return out, nil
}

func mapStringAnyValue(m map[string]any, key string) (map[string]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	out := make(map[string]any, len(mv))
	for k, item := range mv {
		out[k] = item
	}
	return out, nil
}

func mapStringStringValue(m map[string]any, key string) (map[string]string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	out := make(map[string]string, len(mv))
	for k, item := range mv {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%s] must be string, got %T", key, k, item)
		}
		out[k] = s
	}
	return out, nil
}

func mapStringFloat64Value(m map[string]any, key string) (map[string]float64, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	mv, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s must be map[string]any, got %T", key, v)
	}
	out := make(map[string]float64, len(mv))
	for k, item := range mv {
		n, err := toFloat64(item)
		if err != nil {
			return nil, fmt.Errorf("%s[%s]: %w", key, k, err)
		}
		out[k] = n
	}
	return out, nil
}

// mapStringAnySliceValue returns a shallow-copied []map[string]any.
// It copies each top-level map entry, but does not deep-copy nested maps/slices.
func mapStringAnySliceValue(m map[string]any, key string) ([]map[string]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be []any, got %T", key, v)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mv, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s item must be map[string]any, got %T", key, item)
		}
		copied := make(map[string]any, len(mv))
		for k, v := range mv {
			copied[k] = v
		}
		out = append(out, copied)
	}
	return out, nil
}

// mapListValue returns a shallow-copied []map[string]any.
// It copies each top-level map entry, but does not deep-copy nested maps/slices.
func mapListValue(m map[string]any, key string) ([]map[string]any, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return nil, nil
	}
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be []any, got %T", key, v)
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mv, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s item must be map[string]any, got %T", key, item)
		}
		copied := make(map[string]any, len(mv))
		for k, v := range mv {
			copied[k] = v
		}
		out = append(out, copied)
	}
	return out, nil
}

func setMapString(out map[string]any, key, value string) {
	if value != "" {
		out[key] = value
	}
}

func setMapInt(out map[string]any, key string, value int) {
	if value != 0 {
		out[key] = value
	}
}

func setMapBool(out map[string]any, key string, value bool) {
	if value {
		out[key] = value
	}
}

func setMapStringSlice(out map[string]any, key string, value []string) {
	if len(value) > 0 {
		items := make([]any, 0, len(value))
		for _, item := range value {
			items = append(items, item)
		}
		out[key] = items
	}
}
