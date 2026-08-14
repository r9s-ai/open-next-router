package dslconfig

// ProviderObservability contains provider-scoped observation rules.
type ProviderObservability struct {
	UpstreamRequestID *UpstreamRequestIDRule
}

// UpstreamRequestIDRule lists upstream response headers in lookup priority order.
type UpstreamRequestIDRule struct {
	Headers []string
}
