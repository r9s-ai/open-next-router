package dslconfig

// SSEJSONDelIfRule deletes DelPath when CondPath equals Equals (string exact match).
// All paths are restricted to the v0.1 object-path subset (no arrays).
type SSEJSONDelIfRule struct {
	CondPath string
	Equals   string
	DelPath  string
}
