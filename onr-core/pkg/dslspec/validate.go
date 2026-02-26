package dslspec

import (
	"fmt"
	"strings"
)

// ValidateSpec performs basic structural checks on one DSL spec.
func ValidateSpec(spec Spec) error {
	if strings.TrimSpace(spec.Version) == "" {
		return fmt.Errorf("spec version is required")
	}
	if len(spec.Blocks) == 0 {
		return fmt.Errorf("at least one block is required")
	}
	if len(spec.Directives) == 0 {
		return fmt.Errorf("at least one directive is required")
	}

	blockSet := make(map[string]struct{}, len(spec.Blocks))
	for _, b := range spec.Blocks {
		id := strings.TrimSpace(b.ID)
		if id == "" {
			return fmt.Errorf("block id is required")
		}
		if _, ok := blockSet[id]; ok {
			return fmt.Errorf("duplicate block id: %s", id)
		}
		blockSet[id] = struct{}{}
	}

	directiveIDSet := make(map[string]struct{}, len(spec.Directives))
	for _, d := range spec.Directives {
		id := strings.TrimSpace(d.ID)
		if id == "" {
			return fmt.Errorf("directive id is required")
		}
		if _, ok := directiveIDSet[id]; ok {
			return fmt.Errorf("duplicate directive id: %s", id)
		}
		directiveIDSet[id] = struct{}{}

		if strings.TrimSpace(d.Name) == "" {
			return fmt.Errorf("directive %s: name is required", id)
		}
		if len(d.AllowedIn) == 0 {
			return fmt.Errorf("directive %s: allowed_in is required", id)
		}
		for _, block := range d.AllowedIn {
			if _, ok := blockSet[strings.TrimSpace(block)]; !ok {
				return fmt.Errorf("directive %s: unknown block in allowed_in: %s", id, block)
			}
		}
		if err := validateUniqueValues("directive "+id+" modes", d.Modes); err != nil {
			return err
		}
		for i, arg := range d.Args {
			if strings.TrimSpace(arg.Name) == "" {
				return fmt.Errorf("directive %s: arg[%d] name is required", id, i)
			}
			if strings.TrimSpace(arg.Type) == "" {
				return fmt.Errorf("directive %s: arg[%d] type is required", id, i)
			}
			if err := validateUniqueValues("directive "+id+" arg "+arg.Name+" enum", arg.Enum); err != nil {
				return err
			}
		}
	}

	return nil
}

// ValidateLocale checks required locale text coverage for one spec.
func ValidateLocale(spec Spec, bundle LocaleBundle) error {
	if strings.TrimSpace(bundle.Locale) == "" {
		return fmt.Errorf("locale is required")
	}
	if bundle.BlockTitles == nil {
		return fmt.Errorf("block_titles is required")
	}
	if bundle.DirectiveText == nil {
		return fmt.Errorf("directive_text is required")
	}

	for _, b := range spec.Blocks {
		key := strings.TrimSpace(b.ID)
		if key == "" {
			continue
		}
		if strings.TrimSpace(bundle.BlockTitles[key]) == "" {
			return fmt.Errorf("locale %s missing block title for %s", bundle.Locale, key)
		}
	}

	for _, d := range spec.Directives {
		id := strings.TrimSpace(d.ID)
		if id == "" {
			continue
		}
		txt, ok := bundle.DirectiveText[id]
		if !ok {
			return fmt.Errorf("locale %s missing directive text for %s", bundle.Locale, id)
		}
		if strings.TrimSpace(txt.Summary) == "" {
			return fmt.Errorf("locale %s missing summary for directive %s", bundle.Locale, id)
		}
	}
	return nil
}

func validateUniqueValues(name string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		if v == "" {
			return fmt.Errorf("%s contains empty value", name)
		}
		if _, ok := seen[v]; ok {
			return fmt.Errorf("%s contains duplicate value: %s", name, v)
		}
		seen[v] = struct{}{}
	}
	return nil
}
