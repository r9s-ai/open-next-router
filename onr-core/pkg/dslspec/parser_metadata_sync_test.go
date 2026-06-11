package dslspec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type parserDirectiveSource struct {
	block string
	fn    string
	kind  string
}

const (
	parserDirectiveSwitch      = "switch"
	parserDirectiveHandlerMap  = "handler_map"
	parserDirectiveStringEqual = "string_equal"
)

func TestParserDirectivesHaveMetadata(t *testing.T) {
	parsed := collectParserDirectives(t)

	for block, ignored := range parserDirectiveIgnores() {
		for _, name := range ignored {
			delete(parsed[block], name)
		}
	}

	var missing []string
	for block, names := range parsed {
		metadata := stringSet(DirectivesByBlock(block))
		for name := range names {
			if _, ok := metadata[name]; !ok {
				missing = append(missing, block+"."+name)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("parser directives missing dslspec metadata: %s", strings.Join(missing, ", "))
	}
}

func TestAfterReqMapMetadataMatchesParser(t *testing.T) {
	parsed := collectParserDirectives(t)
	assertStringSetEqual(t, "after_req_map parser directives", parsed["after_req_map"], stringSet(DirectivesByBlock("after_req_map")))
}

func collectParserDirectives(t *testing.T) map[string]map[string]struct{} {
	t.Helper()

	files, constants := parseDSLConfigFiles(t)
	out := map[string]map[string]struct{}{}
	for _, source := range parserDirectiveSources() {
		fn := findFuncDecl(files, source.fn)
		if fn == nil {
			t.Fatalf("parser function %s not found", source.fn)
		}
		var names []string
		switch source.kind {
		case parserDirectiveSwitch:
			names = collectSwitchDirectiveNames(fn, constants)
		case parserDirectiveHandlerMap:
			names = collectHandlerMapDirectiveNames(fn)
		case parserDirectiveStringEqual:
			names = collectStringEqualDirectiveNames(fn)
		default:
			t.Fatalf("unsupported parser directive source kind %q", source.kind)
		}
		addDirectiveNames(out, source.block, names)
	}
	return out
}

func parserDirectiveSources() []parserDirectiveSource {
	return []parserDirectiveSource{
		{block: "top", fn: "parseProvidersFromMergedFile", kind: parserDirectiveSwitch},
		{block: "top", fn: "parseGlobalUsageModes", kind: parserDirectiveSwitch},
		{block: "top", fn: "parseGlobalFinishReasonModes", kind: parserDirectiveSwitch},
		{block: "top", fn: "parseGlobalModelsModes", kind: parserDirectiveSwitch},
		{block: "top", fn: "parseGlobalBalanceModes", kind: parserDirectiveSwitch},
		{block: "provider", fn: "parseProviderBody", kind: parserDirectiveSwitch},
		{block: "metadata", fn: "parseMetadataBlock", kind: parserDirectiveSwitch},
		{block: "upstream_config", fn: "parseUpstreamConfigBlock", kind: parserDirectiveStringEqual},
		{block: "auth", fn: "parseAuthPhase", kind: parserDirectiveHandlerMap},
		{block: "request", fn: "parseRequestPhaseWithTransform", kind: parserDirectiveHandlerMap},
		{block: "after_req_map", fn: "parseRequestJSONOpsOnlyBlock", kind: parserDirectiveSwitch},
		{block: "upstream", fn: "parseUpstreamPhase", kind: parserDirectiveSwitch},
		{block: "response", fn: "parseResponsePhase", kind: parserDirectiveSwitch},
		{block: "error", fn: "parseErrorPhase", kind: parserDirectiveSwitch},
		{block: "metrics", fn: "parseMetricsPhase", kind: parserDirectiveSwitch},
		{block: "balance", fn: "parseBalancePhase", kind: parserDirectiveSwitch},
		{block: "models", fn: "parseModelsPhase", kind: parserDirectiveSwitch},
		{block: "usage_mode", fn: "parseUsageModeBlock", kind: parserDirectiveSwitch},
		{block: "finish_reason_mode", fn: "parseFinishReasonModeBlock", kind: parserDirectiveSwitch},
		{block: "models_mode", fn: "parseModelsModeBlock", kind: parserDirectiveSwitch},
		{block: "balance_mode", fn: "parseBalanceModeBlock", kind: parserDirectiveSwitch},
	}
}

func parserDirectiveIgnores() map[string][]string {
	return map[string][]string{
		"balance":  {"used"},
		"metrics":  {"input_tokens", "output_tokens", "cache_read_tokens", "cache_write_tokens", "total_tokens"},
		"upstream": {"query_set"},
	}
}

func parseDSLConfigFiles(t *testing.T) ([]*ast.File, map[string]string) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dslconfigDir := filepath.Join(filepath.Dir(thisFile), "..", "dslconfig")
	paths, err := filepath.Glob(filepath.Join(dslconfigDir, "*.go"))
	if err != nil {
		t.Fatalf("glob dslconfig files: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("no dslconfig files found in %s", dslconfigDir)
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files = append(files, file)
	}
	return files, collectStringConstants(files)
}

func collectStringConstants(files []*ast.File) map[string]string {
	constants := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range valueSpec.Names {
					if i >= len(valueSpec.Values) {
						continue
					}
					value, ok := stringLiteralValue(valueSpec.Values[i])
					if ok {
						constants[name.Name] = value
					}
				}
			}
		}
	}
	return constants
}

func findFuncDecl(files []*ast.File, name string) *ast.FuncDecl {
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Name.Name == name {
				return fn
			}
		}
	}
	return nil
}

func collectSwitchDirectiveNames(fn *ast.FuncDecl, constants map[string]string) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		stmt, ok := n.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range stmt.List {
			value, ok := directiveExprValue(expr, constants)
			if ok {
				names = append(names, value)
			}
		}
		return true
	})
	return names
}

func collectHandlerMapDirectiveNames(fn *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != "handlers" {
			return true
		}
		composite, ok := assign.Rhs[0].(*ast.CompositeLit)
		if !ok || !isStringKeyedMap(composite.Type) {
			return true
		}
		for _, elt := range composite.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			value, ok := stringLiteralValue(kv.Key)
			if ok {
				names = append(names, value)
			}
		}
		return true
	})
	return names
}

func collectStringEqualDirectiveNames(fn *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		expr, ok := n.(*ast.BinaryExpr)
		if !ok || expr.Op != token.EQL {
			return true
		}
		if isTokTextSelector(expr.X) {
			if value, ok := stringLiteralValue(expr.Y); ok {
				names = append(names, value)
			}
		}
		if isTokTextSelector(expr.Y) {
			if value, ok := stringLiteralValue(expr.X); ok {
				names = append(names, value)
			}
		}
		return true
	})
	return names
}

func directiveExprValue(expr ast.Expr, constants map[string]string) (string, bool) {
	if value, ok := stringLiteralValue(expr); ok {
		return value, true
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}
	value, ok := constants[ident.Name]
	return value, ok
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}

func isStringKeyedMap(expr ast.Expr) bool {
	mapType, ok := expr.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mapType.Key.(*ast.Ident)
	return ok && key.Name == "string"
}

func isTokTextSelector(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "text" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == "tok"
}

func addDirectiveNames(target map[string]map[string]struct{}, block string, names []string) {
	if _, ok := target[block]; !ok {
		target[block] = map[string]struct{}{}
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		target[block][name] = struct{}{}
	}
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func assertStringSetEqual(t *testing.T, name string, got, want map[string]struct{}) {
	t.Helper()
	var missing []string
	for key := range want {
		if _, ok := got[key]; !ok {
			missing = append(missing, key)
		}
	}
	var extra []string
	for key := range got {
		if _, ok := want[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("%s mismatch: missing=%v extra=%v", name, missing, extra)
	}
}
