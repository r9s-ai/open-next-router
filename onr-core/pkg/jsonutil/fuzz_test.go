package jsonutil

import (
	"encoding/json"
	"reflect"
	"testing"
)

func FuzzJSONPathVisitorsAgree(f *testing.F) {
	for _, seed := range []struct {
		body string
		path string
	}{
		{`{"usage":{"input_tokens":3}}`, `$.usage.input_tokens`},
		{`{"items":[{"tokens":1},{"tokens":2}]}`, `$.items[*].tokens`},
		{`{"items":[{"kind":"output","tokens":2}]}`, `$.items[?(@.kind=="output")].tokens`},
		{`[]`, `$.items[0]`},
	} {
		f.Add([]byte(seed.body), seed.path)
	}

	f.Fuzz(func(t *testing.T, body []byte, path string) {
		var root any
		if err := json.Unmarshal(body, &root); err != nil {
			return
		}

		values, matched := GetValuesByPath(root, path)
		visited := make([]any, 0)
		visitedMatched := VisitValuesByPath(root, path, func(value any) {
			visited = append(visited, value)
		})
		if matched != visitedMatched {
			t.Fatalf("GetValuesByPath matched=%v, VisitValuesByPath matched=%v for %q", matched, visitedMatched, path)
		}
		if !matched {
			return
		}
		if !reflect.DeepEqual(values, visited) {
			t.Fatalf("GetValuesByPath=%#v, VisitValuesByPath=%#v for %q", values, visited, path)
		}
	})
}
