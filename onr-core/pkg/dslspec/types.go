package dslspec

// Spec is the language-agnostic structural source for DSL docs and metadata generation.
type Spec struct {
	Version    string          `yaml:"version"`
	Blocks     []BlockSpec     `yaml:"blocks"`
	Directives []DirectiveSpec `yaml:"directives"`
}

// BlockSpec defines one DSL block scope.
type BlockSpec struct {
	ID   string `yaml:"id"`
	Kind string `yaml:"kind"`
}

// DirectiveSpec defines one DSL directive in normalized form.
type DirectiveSpec struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	AllowedIn   []string           `yaml:"allowed_in"`
	Kind        string             `yaml:"kind"`
	Args        []DirectiveArgSpec `yaml:"args,omitempty"`
	Modes       []string           `yaml:"modes,omitempty"`
	Repeatable  bool               `yaml:"repeatable"`
	Constraints []string           `yaml:"constraints,omitempty"`
}

// DirectiveArgSpec defines one positional argument.
type DirectiveArgSpec struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Required bool     `yaml:"required"`
	Enum     []string `yaml:"enum,omitempty"`
}

// LocaleBundle stores display text for one locale.
type LocaleBundle struct {
	Locale        string                   `yaml:"locale"`
	BlockTitles   map[string]string        `yaml:"block_titles"`
	DirectiveText map[string]DirectiveText `yaml:"directive_text"`
}

// DirectiveText stores i18n strings for one directive.
type DirectiveText struct {
	Summary string `yaml:"summary"`
	Details string `yaml:"details,omitempty"`
	Example string `yaml:"example,omitempty"`
}
