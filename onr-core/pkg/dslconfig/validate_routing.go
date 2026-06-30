package dslconfig

import (
	"fmt"
	"strings"
)

func validateProviderRouting(path, providerName string, routing ProviderRouting) error {
	transport := strings.ToLower(strings.TrimSpace(routing.Transport))
	if transport == "" {
		transport = "http"
	}
	switch transport {
	case "http", "aws_sdk":
		// ok
	default:
		return validationIssue(
			fmt.Errorf("provider %q in %q: upstream_config unsupported transport %q", providerName, path, routing.Transport),
			"upstream_config",
			"transport",
		)
	}
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
		if transport == "aws_sdk" {
			if len(m.QueryPairs) > 0 || len(m.QueryDels) > 0 {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s bedrock_runtime does not support query routing", providerName, path, scope),
					scope,
					"set_query",
				)
			}
			if strings.TrimSpace(m.SetPath) == "" {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s set_path is required for bedrock_runtime", providerName, path, scope),
					scope,
					"set_path",
				)
			}
			kind, err := validateBedrockRuntimePathExpr(m.SetPath)
			if err != nil {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s invalid bedrock_runtime set_path: %w", providerName, path, scope, err),
					scope,
					"set_path",
				)
			}
			isStreamMatch := m.Stream != nil && *m.Stream
			if kind == "stream" && !isStreamMatch {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s invoke-with-response-stream path requires match stream = true", providerName, path, scope),
					scope,
					"set_path",
				)
			}
			if kind == "invoke" && isStreamMatch {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s invoke path requires match stream = false", providerName, path, scope),
					scope,
					"set_path",
				)
			}
		}
		for key, value := range m.QueryPairs {
			if err := ValidateStringExpr(value); err != nil {
				return validationIssue(
					fmt.Errorf("provider %q in %q: %s invalid set_query %q expression: %w", providerName, path, scope, key, err),
					scope,
					"set_query",
				)
			}
		}
	}
	return nil
}

func validateBedrockRuntimePathExpr(expr string) (string, error) {
	raw := strings.TrimSpace(expr)
	var path string
	switch {
	case isQuotedStringExpr(raw):
		path = unquoteString(raw)
	case strings.HasPrefix(raw, "template(") && strings.HasSuffix(raw, ")"):
		tmpl, err := validateTemplateExpr(raw)
		if err != nil {
			return "", err
		}
		path = tmpl
	case strings.HasPrefix(raw, "/"):
		path = raw
	default:
		return "", fmt.Errorf("expected literal or template path")
	}
	if !strings.HasPrefix(path, "/model/") {
		if strings.HasPrefix(path, "/") {
			return "http-passthrough", nil
		}
		return "", fmt.Errorf("path must be an absolute path")
	}
	modelPart := strings.TrimPrefix(path, "/model/")
	switch {
	case strings.HasSuffix(modelPart, "/invoke-with-response-stream"):
		modelID := strings.TrimSuffix(modelPart, "/invoke-with-response-stream")
		if strings.TrimSpace(modelID) == "" {
			return "", fmt.Errorf("model id segment is empty")
		}
		return "stream", nil
	case strings.HasSuffix(modelPart, "/invoke"):
		modelID := strings.TrimSuffix(modelPart, "/invoke")
		if strings.TrimSpace(modelID) == "" {
			return "", fmt.Errorf("model id segment is empty")
		}
		return "invoke", nil
	default:
		return "", fmt.Errorf("path must end with /invoke or /invoke-with-response-stream")
	}
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

// ValidatePathOrURLExpr validates a DSL expression whose literal template must be a path or absolute URL.
func ValidatePathOrURLExpr(expr string) error {
	raw := strings.TrimSpace(expr)
	if raw == "" {
		return fmt.Errorf("expression is empty")
	}
	if isQuotedStringExpr(raw) {
		return validateURLPathLiteral(unquoteString(raw))
	}
	if validateURLPathLiteral(raw) == nil {
		if strings.Contains(raw, "$") {
			return fmt.Errorf("unquoted path/url literals cannot contain '$'; use template(...) for variables")
		}
		return nil
	}
	if strings.HasPrefix(raw, "concat(") && strings.HasSuffix(raw, ")") {
		args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(raw, "concat("), ")"))
		if len(args) == 0 {
			return fmt.Errorf("concat requires at least one argument")
		}
		if err := ValidatePathOrURLExpr(args[0]); err != nil {
			return fmt.Errorf("concat argument 0 must be path/url-shaped: %w", err)
		}
		for i, arg := range args[1:] {
			if err := ValidateStringExpr(arg); err != nil {
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
		return validateURLPathLiteral(tmpl)
	}
	if isBuiltinStringVariable(raw) {
		return fmt.Errorf("bare variables are not valid path/url expressions; embed variables in template(...) or concat(...) with a path/url prefix")
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
	return ValidateStringExpr(expr)
}

// ValidateStringExpr validates a DSL string expression.
func ValidateStringExpr(expr string) error {
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
