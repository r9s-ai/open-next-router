package apitypes

import (
	"encoding/json"
	"fmt"
)

// JSONObject is a generic JSON object used as a typed boundary in pkg APIs.
type JSONObject map[string]any

// ParseJSONObject parses bytes into a JSON object.
func ParseJSONObject(b []byte, what string) (JSONObject, error) {
	var obj any
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil, fmt.Errorf("parse %s json: %w", what, err)
	}
	root, _ := obj.(map[string]any)
	if root == nil {
		return nil, fmt.Errorf("%s json is not an object", what)
	}
	return JSONObject(root), nil
}

// Marshal marshals the object to JSON bytes.
func (o JSONObject) Marshal() ([]byte, error) {
	return json.Marshal(map[string]any(o))
}
