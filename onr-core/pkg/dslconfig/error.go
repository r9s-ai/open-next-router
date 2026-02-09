package dslconfig

// Error phase (v0.1) only needs a "selectable single directive + mode" structure.
// To avoid duplicating logic with the response phase, we reuse the same types and selection behavior.
type ErrorDirective = ResponseDirective

type ProviderError = ProviderResponse

type MatchError = MatchResponse
