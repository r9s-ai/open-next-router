package dslconfig

import (
	"path/filepath"
	"testing"
)

func TestValidateProvidersDir_ConfigProviders(t *testing.T) {
	dir := filepath.Join("..", "..", "config", "providers")
	if _, err := ValidateProvidersDir(dir); err != nil {
		t.Fatalf("validate providers dir failed: %v", err)
	}
}

