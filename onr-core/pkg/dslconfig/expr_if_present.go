package dslconfig

import (
	"fmt"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/jsonutil"
)

// exprIfPresentPrefix is the call form of the if_present expression:
//
//	if_present("<jsonpath>", <then-expr>, <else-expr>)
//
// It selects between two expressions based on whether the client's request body
// carries a value at the given path. Upstreams sometimes expose the same logical
// operation at different endpoints depending on the shape of the request — PPIO
// splits video generation into .../<model>-text2video and .../<model>-img2video
// by whether a reference image was sent — and the routing block otherwise has no
// way to see the body at all: its variables are limited to channel, credential
// and model metadata.
const exprIfPresentPrefix = "if_present("

// evalIfPresentExpr evaluates an if_present(...) call. Both branches are
// evaluated as ordinary string expressions, so they compose with concat,
// template and the builtin variables.
func evalIfPresentExpr(raw string, meta *dslmeta.Meta, requireNonEmptyVariables bool) (string, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(raw, exprIfPresentPrefix), ")")
	args := splitTopLevelArgs(inner)
	if len(args) != 3 {
		return "", fmt.Errorf("if_present expects 3 arguments (path, then, else), got %d", len(args))
	}
	pathArg := strings.TrimSpace(args[0])
	if !isQuotedStringExpr(pathArg) {
		return "", fmt.Errorf("if_present path must be a quoted json path")
	}
	path := strings.TrimSpace(unquoteString(pathArg))
	if path == "" {
		return "", fmt.Errorf("if_present path must not be empty")
	}

	branch := args[2]
	if meta != nil && requestPathPresent(meta, path) {
		branch = args[1]
	}
	return evalStringExprValue(branch, meta, requireNonEmptyVariables)
}

// requestPathPresent reports whether the client request carries content at the
// path, looking at both the request root and — for multipart requests — the
// uploaded file fields, which never reach the root.
func requestPathPresent(meta *dslmeta.Meta, path string) bool {
	if requestPathCarriesValue(meta.RequestRoot(), path) {
		return true
	}
	// Only a top-level field can name an upload, which is how multipart form
	// fields are addressed: "$.input_reference".
	if field, ok := strings.CutPrefix(path, "$."); ok && !strings.ContainsAny(field, ".[") {
		return meta.RequestHasUploadedFile(field)
	}
	return false
}

// requestPathCarriesValue reports whether the request body actually carries
// content at the path. Mere presence of the key is not enough: an explicit null,
// an empty array and an empty string all mean the caller sent nothing, and the
// Go adaptors this replaces branch on emptiness (len(InputReference) == 0), not
// on whether the field was written.
func requestPathCarriesValue(root map[string]any, path string) bool {
	values, ok := jsonutil.GetValuesByPath(root, path)
	if !ok {
		return false
	}
	for _, v := range values {
		switch typed := v.(type) {
		case nil:
			continue
		case []any:
			if len(typed) > 0 {
				return true
			}
		case map[string]any:
			if len(typed) > 0 {
				return true
			}
		case string:
			if strings.TrimSpace(typed) != "" {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// isIfPresentExpr reports whether the expression is an if_present(...) call.
func isIfPresentExpr(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, exprIfPresentPrefix) && strings.HasSuffix(trimmed, ")")
}

// validateIfPresentExpr checks an if_present call at config load time so a
// malformed one fails the provider file rather than silently routing every
// request to the else branch.
func validateIfPresentExpr(raw string) error {
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), exprIfPresentPrefix), ")")
	args := splitTopLevelArgs(inner)
	if len(args) != 3 {
		return fmt.Errorf("if_present expects 3 arguments (path, then, else), got %d", len(args))
	}
	pathArg := strings.TrimSpace(args[0])
	if !isQuotedStringExpr(pathArg) {
		return fmt.Errorf("if_present path must be a quoted json path")
	}
	path := strings.TrimSpace(unquoteString(pathArg))
	if path == "" {
		return fmt.Errorf("if_present path must not be empty")
	}
	if !strings.HasPrefix(path, "$") {
		return fmt.Errorf("if_present path must start with $, got %q", path)
	}
	return nil
}

// validateIfPresentBranches validates an if_present call used where any string
// expression is allowed, checking the call shape and both branches.
func validateIfPresentBranches(raw string) error {
	if err := validateIfPresentExpr(raw); err != nil {
		return err
	}
	args := splitTopLevelArgs(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), exprIfPresentPrefix), ")"))
	for i, arg := range args[1:] {
		if err := ValidateStringExpr(arg); err != nil {
			return fmt.Errorf("if_present branch %d: %w", i+1, err)
		}
	}
	return nil
}
