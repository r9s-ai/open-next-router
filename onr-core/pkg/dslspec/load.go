package dslspec

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

//go:embed schema.yaml i18n/*.yaml
var builtinFS embed.FS

// LoadBuiltinSpec loads the embedded structural DSL spec.
func LoadBuiltinSpec() (Spec, error) {
	return loadSpecFS(builtinFS, "schema.yaml")
}

// LoadBuiltinLocale loads one embedded locale bundle (for example: en, zh-CN).
func LoadBuiltinLocale(locale string) (LocaleBundle, error) {
	return loadLocaleFS(builtinFS, "i18n/"+locale+".yaml")
}

func loadSpecFS(fsys fs.FS, path string) (Spec, error) {
	var out Spec
	if err := loadYAMLFileFS(fsys, path, &out); err != nil {
		return Spec{}, err
	}
	return out, nil
}

func loadLocaleFS(fsys fs.FS, path string) (LocaleBundle, error) {
	var out LocaleBundle
	if err := loadYAMLFileFS(fsys, path, &out); err != nil {
		return LocaleBundle{}, err
	}
	return out, nil
}

func loadYAMLFileFS(fsys fs.FS, path string, out any) error {
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
