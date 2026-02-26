package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
)

const (
	beginMarker = "<!-- BEGIN GENERATED: dslspec-reference -->"
	endMarker   = "<!-- END GENERATED: dslspec-reference -->"
)

type targetDoc struct {
	Locale string
	Path   string
}

func main() {
	repoRoot := flag.String("repo-root", ".", "repository root directory")
	flag.Parse()

	spec, err := dslspec.LoadBuiltinSpec()
	if err != nil {
		fatalf("load builtin spec: %v", err)
	}
	if err := dslspec.ValidateSpec(spec); err != nil {
		fatalf("validate builtin spec: %v", err)
	}

	targets := []targetDoc{
		{Locale: "en", Path: "DSL_SYNTAX.md"},
		{Locale: "zh-CN", Path: "DSL_SYNTAX_CN.md"},
	}

	for _, target := range targets {
		localeBundle, err := dslspec.LoadBuiltinLocale(target.Locale)
		if err != nil {
			fatalf("load locale %s: %v", target.Locale, err)
		}
		if err := dslspec.ValidateLocale(spec, localeBundle); err != nil {
			fatalf("validate locale %s: %v", target.Locale, err)
		}

		body := renderReference(spec, localeBundle)
		if err := rewriteDoc(filepath.Join(*repoRoot, target.Path), body); err != nil {
			fatalf("rewrite %s: %v", target.Path, err)
		}
	}
}

func rewriteDoc(path, generatedBody string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	updated, err := replaceGeneratedRegion(string(b), generatedBody)
	if err != nil {
		return err
	}
	if updated == string(b) {
		return nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func replaceGeneratedRegion(content, generatedBody string) (string, error) {
	start := strings.Index(content, beginMarker)
	if start < 0 {
		return "", fmt.Errorf("begin marker not found: %s", beginMarker)
	}
	end := strings.Index(content, endMarker)
	if end < 0 {
		return "", fmt.Errorf("end marker not found: %s", endMarker)
	}
	if end < start {
		return "", fmt.Errorf("invalid marker order")
	}

	newRegion := beginMarker + "\n" + strings.TrimRight(generatedBody, "\n") + "\n" + endMarker
	oldRegion := content[start : end+len(endMarker)]
	return strings.Replace(content, oldRegion, newRegion, 1), nil
}

func renderReference(spec dslspec.Spec, locale dslspec.LocaleBundle) string {
	zh := strings.EqualFold(locale.Locale, "zh-CN")

	directivesByBlock := make(map[string][]dslspec.DirectiveSpec, len(spec.Blocks))
	for _, d := range spec.Directives {
		for _, block := range d.AllowedIn {
			blockID := strings.TrimSpace(block)
			if blockID == "" {
				continue
			}
			directivesByBlock[blockID] = append(directivesByBlock[blockID], d)
		}
	}
	for blockID := range directivesByBlock {
		sort.Slice(directivesByBlock[blockID], func(i, j int) bool {
			li := directivesByBlock[blockID][i]
			lj := directivesByBlock[blockID][j]
			if li.Name == lj.Name {
				return li.ID < lj.ID
			}
			return li.Name < lj.Name
		})
	}

	var out bytes.Buffer
	if zh {
		out.WriteString("> 本节由 `dslspec` 自动生成，请勿手工编辑。\n\n")
	} else {
		out.WriteString("> This section is auto-generated from `dslspec`. Do not edit manually.\n\n")
	}

	for _, block := range spec.Blocks {
		items := directivesByBlock[block.ID]
		if len(items) == 0 {
			continue
		}
		blockTitle := strings.TrimSpace(locale.BlockTitles[block.ID])
		if blockTitle == "" {
			blockTitle = block.ID
		}
		if zh {
			out.WriteString("### ")
			out.WriteString(blockTitle)
			out.WriteString("（`")
			out.WriteString(block.ID)
			out.WriteString("`）\n\n")
			out.WriteString("| 指令 | 参数 | 模式 | 可重复 | 说明 |\n")
			out.WriteString("|---|---|---|---|---|\n")
		} else {
			out.WriteString("### ")
			out.WriteString(blockTitle)
			out.WriteString(" (`")
			out.WriteString(block.ID)
			out.WriteString("`)\n\n")
			out.WriteString("| Directive | Args | Modes | Repeatable | Summary |\n")
			out.WriteString("|---|---|---|---|---|\n")
		}

		for _, item := range items {
			summary := strings.TrimSpace(locale.DirectiveText[item.ID].Summary)
			if summary == "" {
				summary = "-"
			}
			out.WriteString("| `")
			out.WriteString(item.Name)
			out.WriteString("` | ")
			out.WriteString(markdownEscape(fieldArgs(item.Args)))
			out.WriteString(" | ")
			out.WriteString(markdownEscape(fieldModes(item.Modes)))
			out.WriteString(" | ")
			out.WriteString(boolWord(item.Repeatable, zh))
			out.WriteString(" | ")
			out.WriteString(markdownEscape(summary))
			out.WriteString(" |\n")
		}
		out.WriteString("\n")
	}
	return out.String()
}

func fieldArgs(args []dslspec.DirectiveArgSpec) string {
	if len(args) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		name := strings.TrimSpace(arg.Name)
		typ := strings.TrimSpace(arg.Type)
		if name == "" {
			name = "arg"
		}
		if typ == "" {
			typ = "value"
		}
		s := name + ":" + typ
		if !arg.Required {
			s += "?"
		}
		if len(arg.Enum) > 0 {
			s += "{" + strings.Join(arg.Enum, "|") + "}"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func fieldModes(modes []string) string {
	if len(modes) == 0 {
		return "-"
	}
	return strings.Join(modes, ", ")
}

func markdownEscape(s string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", " ")
	return replacer.Replace(s)
}

func boolWord(v bool, zh bool) string {
	if zh {
		if v {
			return "是"
		}
		return "否"
	}
	if v {
		return "yes"
	}
	return "no"
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "onr-dsldocgen: "+format+"\n", args...)
	os.Exit(1)
}
