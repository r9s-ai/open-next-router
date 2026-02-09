package dslconfig

import (
	"path/filepath"
	"testing"
)

func TestValidateProvidersDir_ConfigProviders(t *testing.T) {
	candidates := []string{
		filepath.Join("..", "..", "..", "config", "providers"),
		filepath.Join("..", "..", "config", "providers"),
	}
	for _, dir := range candidates {
		if _, err := ValidateProvidersDir(dir); err == nil {
			return
		}
	}
	t.Fatalf("validate providers dir failed for all candidates: %v", candidates)
}
