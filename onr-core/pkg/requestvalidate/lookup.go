package requestvalidate

import (
	"net/http"
	"net/url"
)

// lookupBody walks the parsed JSON object root along pre-parsed path parts.
// It copies no intermediate objects; a non-object intermediate value counts as missing.
func lookupBody(root map[string]any, parts []string) (any, bool) {
	cur := root
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok {
			return nil, false
		}
		m, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		cur = m
	}
	value, ok := cur[parts[len(parts)-1]]
	return value, ok
}

// lookupHeader indexes headers by the pre-canonicalized name, avoiding the
// per-request canonicalization Header.Get would perform.
func lookupHeader(headers http.Header, canonicalName string) (any, bool) {
	if headers == nil {
		return nil, false
	}
	values, ok := headers[canonicalName]
	if !ok || len(values) == 0 {
		return nil, false
	}
	return values[0], true
}

func lookupQuery(query url.Values, name string) (any, bool) {
	values, ok := query[name]
	if !ok || len(values) == 0 {
		return nil, false
	}
	return values[0], true
}
