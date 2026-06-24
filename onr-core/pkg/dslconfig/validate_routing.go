package dslconfig

import (
	"fmt"
	"strings"
)

func validateProviderRouting(path, providerName string, routing ProviderRouting) error {
	for i, m := range routing.Matches {
		scope := fmt.Sprintf("match[%d].upstream", i)
		if strings.TrimSpace(m.SetPath) != "" {
			if err := validateSetPathExpr(m.SetPath); err != nil {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s invalid set_path expression: %w", providerName, path, scope, err),
					scope,
					"set_path",
				)
			}
		}
	}
	return nil
}

func validateSetPathExpr(expr string) error {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return fmt.Errorf("expression is empty")
	}
	if isQuotedStringExpr(raw) {
		return validatePathLiteral(unquoteString(raw))
	}
	if strings.HasPrefix(raw, "/") {
		if strings.Contains(raw, "$") {
			return fmt.Errorf("unquoted path literals cannot contain '$'; use template(...) for variables")
		}
		return nil
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")"))
		if len(args) == 0 {
			return fmt.Errorf("concat requires at least one argument")
		}
		if err := validateSetPathExpr(args[0]); err != nil {
			return fmt.Errorf("concat argument 0 must be path-shaped: %w", err)
		}
		for i, arg := range args[1:] {
			if err := validateStringExpr(arg); err != nil {
				return fmt.Errorf("concat argument %d: %w", i+1, err)
			}
		}
		return nil
	}
	if strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")") {
		tmpl, err := validateTemplateExpr(raw)
		if err != nil {
			return err
		}
		return validatePathLiteral(tmpl)
	}
	if isBuiltinStringVariable(raw) {
		return fmt.Errorf("bare variables are not valid set_path expressions; embed variables in template(...) or concat(...) with a '/' prefix")
	}
	return fmt.Errorf("unsupported expression %q", raw)
}

func validateTemplateExpr(raw string) (string, error) {
	args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "template("), ")"))
	if len(args) != 1 {
		return "", fmt.Errorf("template requires exactly one string literal argument")
	}
	arg := strings.TrimSpace(args[0])
	if !isQuotedStringExpr(arg) {
		return "", fmt.Errorf("template requires exactly one string literal argument")
	}
	tmpl := unquoteString(arg)
	if err := validateTemplateString(tmpl); err != nil {
		return "", err
	}
	return tmpl, nil
}

func validatePathLiteral(path string) error {
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path must start with '/', got %q", path)
	}
	return nil
}

func validateStringExpr(expr string) error {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return fmt.Errorf("expression is empty")
	}
	if isQuotedStringExpr(raw) || isBuiltinStringVariable(raw) {
		return nil
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")"))
		if len(args) == 0 {
			return fmt.Errorf("concat requires at least one argument")
		}
		for i, arg := range args {
			if err := validateStringExpr(arg); err != nil {
				return fmt.Errorf("concat argument %d: %w", i, err)
			}
		}
		return nil
	}
	if strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")") {
		_, err := validateTemplateExpr(raw)
		return err
	}
	return fmt.Errorf("unsupported expression %q", raw)
}

func validateTemplateString(tmpl string) error {
	for i := 0; i < len(tmpl); {
		if strings.HasPrefix(tmpl[i:], `\${`) {
			i += len(`\${`)
			continue
		}
		if !strings.HasPrefix(tmpl[i:], "${") {
			i++
			continue
		}
		end := strings.IndexByte(tmpl[i+2:], '}')
		if end < 0 {
			return fmt.Errorf("template placeholder is missing closing '}'")
		}
		name := strings.TrimSpace(tmpl[i+2 : i+2+end])
		if _, ok := normalizeTemplateVariable(name); !ok {
			return fmt.Errorf("unsupported template variable %q", name)
		}
		i += 2 + end + 1
	}
	return nil
}
