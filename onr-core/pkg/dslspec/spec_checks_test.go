package dslspec

import "testing"

func TestBuiltinSpec_Validate(t *testing.T) {
	spec, err := LoadBuiltinSpec()
	if err != nil {
		t.Fatalf("load builtin spec: %v", err)
	}
	if err := ValidateSpec(spec); err != nil {
		t.Fatalf("validate builtin spec: %v", err)
	}
}

func TestSpecLocaleCheck_BuiltinBundles(t *testing.T) {
	spec, err := LoadBuiltinSpec()
	if err != nil {
		t.Fatalf("load builtin spec: %v", err)
	}
	for _, locale := range []string{"en", "zh-CN"} {
		bundle, err := LoadBuiltinLocale(locale)
		if err != nil {
			t.Fatalf("load builtin locale %s: %v", locale, err)
		}
		if err := ValidateLocale(spec, bundle); err != nil {
			t.Fatalf("validate locale %s: %v", locale, err)
		}
	}
}
