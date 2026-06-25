package dslmetadata

import "encoding/json"

func CanonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func EqualCanonical(a, b any) bool {
	left, err := CanonicalJSON(a)
	if err != nil {
		return false
	}
	right, err := CanonicalJSON(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}
