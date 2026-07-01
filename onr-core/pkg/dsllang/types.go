package dsllang

// Position is a zero-based text position.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a zero-based text range.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Diagnostic describes a DSL syntax or semantic issue.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

// MarkupContent describes editor-facing rich text content.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover describes hover documentation for an editor position.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
	Word     string        `json:"word,omitempty"`
	Block    string        `json:"block,omitempty"`
}

type formattingOptions struct {
	TabSize      int  `json:"tabSize"`
	InsertSpaces bool `json:"insertSpaces"`
}
