package dsllang

// CollectDiagnostics returns DSL syntax and semantic diagnostics for text.
func CollectDiagnostics(uri, text string) []Diagnostic {
	return collectDiagnostics(uri, text)
}

// AnalyzeSyntax returns syntax-only diagnostics for DSL text.
func AnalyzeSyntax(text string) []Diagnostic {
	return analyze(text)
}

// AnalyzeSemanticModes returns diagnostics for static directive mode values.
func AnalyzeSemanticModes(text string) []Diagnostic {
	return analyzeSemanticModes(text)
}

// AnalyzeSemantic returns diagnostics from full DSL semantic validation.
func AnalyzeSemantic(uri, text string) []Diagnostic {
	return analyzeSemantic(uri, text)
}

// SemanticDirectivePosition returns the best directive position for a semantic
// validation message.
func SemanticDirectivePosition(text, msg string) (int, int, bool) {
	return semanticDirectivePosition(text, msg)
}

// SemanticDirectivePositionWithScope returns the best directive position for a
// structured semantic validation issue.
func SemanticDirectivePositionWithScope(text, directive, scope string) (int, int, bool) {
	return semanticDirectivePositionWithScope(text, directive, scope)
}

// DiagnosticFromSemanticMessage returns a diagnostic derived from a semantic
// validation message.
func DiagnosticFromSemanticMessage(text, msg string) Diagnostic {
	return diagnosticFromSemanticMessage(text, msg)
}

// ProviderNameFromURI infers a provider name from a file URI or absolute path.
func ProviderNameFromURI(uri string) string {
	return providerNameFromURI(uri)
}

// SemanticTokenLegend describes token type indexes used by CollectSemanticTokens.
type SemanticTokenLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// SemanticTokens is encoded using the LSP semanticTokens/full integer format.
type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

// CollectSemanticTokens returns semantic token data for DSL text.
func CollectSemanticTokens(text string) SemanticTokens {
	tokens := semanticTokensFull(text)
	return SemanticTokens(tokens)
}

// CollectHover returns directive hover documentation for a text position.
func CollectHover(text string, pos Position) (*Hover, bool) {
	return collectHover(text, pos)
}

// CollectSemanticTokenLegend returns the token legend for CollectSemanticTokens.
func CollectSemanticTokenLegend() SemanticTokenLegend {
	return SemanticTokenLegend{
		TokenTypes:     append([]string(nil), semanticTokenLegendTypes...),
		TokenModifiers: []string{},
	}
}

// EndPosition returns the zero-based document end position.
func EndPosition(text string) Position {
	return endPosition(text)
}

// Max returns the larger integer.
func Max(a, b int) int {
	return max(a, b)
}
