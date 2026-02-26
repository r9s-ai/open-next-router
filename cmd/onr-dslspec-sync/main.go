package main

import (
	"bytes"
	"cmp"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	dslconfig "github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslspec"
	"gopkg.in/yaml.v3"
)

type directiveAgg struct {
	d         dslspec.DirectiveSpec
	modeSet   map[string]struct{}
	blockSet  map[string]struct{}
	argsByIdx map[int]dslspec.DirectiveArgSpec
}

func main() {
	repoRoot := flag.String("repo-root", ".", "repository root directory")
	flag.Parse()

	schemaPath := filepath.Join(*repoRoot, "onr-core/pkg/dslspec/schema.yaml")
	enPath := filepath.Join(*repoRoot, "onr-core/pkg/dslspec/i18n/en.yaml")
	zhPath := filepath.Join(*repoRoot, "onr-core/pkg/dslspec/i18n/zh-CN.yaml")

	existingSpec, err := loadSpecFile(schemaPath)
	if err != nil {
		fatalf("load schema: %v", err)
	}
	existingEN, err := loadLocaleFile(enPath)
	if err != nil {
		fatalf("load en locale: %v", err)
	}
	existingZH, err := loadLocaleFile(zhPath)
	if err != nil {
		fatalf("load zh-CN locale: %v", err)
	}

	nextSpec := buildSpec(existingSpec, dslconfig.DirectiveMetadataList())
	if err := dslspec.ValidateSpec(nextSpec); err != nil {
		fatalf("validate generated spec: %v", err)
	}

	nextEN := buildLocale(nextSpec, existingEN, false)
	if err := dslspec.ValidateLocale(nextSpec, nextEN); err != nil {
		fatalf("validate generated en locale: %v", err)
	}
	nextZH := buildLocale(nextSpec, existingZH, true)
	if err := dslspec.ValidateLocale(nextSpec, nextZH); err != nil {
		fatalf("validate generated zh-CN locale: %v", err)
	}

	if err := os.WriteFile(schemaPath, renderSpecYAML(nextSpec), 0o644); err != nil {
		fatalf("write schema: %v", err)
	}
	if err := os.WriteFile(enPath, renderLocaleYAML(nextSpec, nextEN), 0o644); err != nil {
		fatalf("write en locale: %v", err)
	}
	if err := os.WriteFile(zhPath, renderLocaleYAML(nextSpec, nextZH), 0o644); err != nil {
		fatalf("write zh-CN locale: %v", err)
	}
}

func loadSpecFile(path string) (dslspec.Spec, error) {
	var out dslspec.Spec
	if err := loadYAML(path, &out); err != nil {
		return dslspec.Spec{}, err
	}
	return out, nil
}

func loadLocaleFile(path string) (dslspec.LocaleBundle, error) {
	var out dslspec.LocaleBundle
	if err := loadYAML(path, &out); err != nil {
		return dslspec.LocaleBundle{}, err
	}
	return out, nil
}

func loadYAML(path string, out any) error {
	b, err := os.ReadFile(path)
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

func buildSpec(existing dslspec.Spec, meta []dslconfig.DirectiveMetadata) dslspec.Spec {
	aggByName := map[string]*directiveAgg{}
	blockSet := map[string]struct{}{
		"top": {},
	}

	existingByID := make(map[string]dslspec.DirectiveSpec, len(existing.Directives))
	for _, d := range existing.Directives {
		existingByID[d.ID] = d
	}

	for _, item := range meta {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		block := normalizeBlock(item.Block)
		if block != "" {
			blockSet[block] = struct{}{}
		}
		a, ok := aggByName[name]
		if !ok {
			a = &directiveAgg{
				d: dslspec.DirectiveSpec{
					ID:         name,
					Name:       name,
					Kind:       "statement",
					Repeatable: false,
				},
				modeSet:   map[string]struct{}{},
				blockSet:  map[string]struct{}{},
				argsByIdx: map[int]dslspec.DirectiveArgSpec{},
			}
			if old, has := existingByID[name]; has {
				a.d.Repeatable = old.Repeatable
				a.d.Constraints = append([]string(nil), old.Constraints...)
				if len(old.Args) > 0 {
					for idx, arg := range old.Args {
						a.argsByIdx[idx] = arg
					}
				}
			}
			aggByName[name] = a
		}
		if block != "" {
			a.blockSet[block] = struct{}{}
		}
		for _, m := range item.Modes {
			mm := strings.TrimSpace(m)
			if mm == "" {
				continue
			}
			a.modeSet[mm] = struct{}{}
		}
		for idx, arg := range item.Args {
			if len(arg.Enum) == 0 {
				continue
			}
			cur, ok := a.argsByIdx[idx]
			if !ok {
				cur = dslspec.DirectiveArgSpec{
					Name:     strings.TrimSpace(arg.Name),
					Type:     "enum",
					Required: true,
				}
				if cur.Name == "" {
					cur.Name = "arg"
				}
			}
			enumSet := map[string]struct{}{}
			for _, old := range cur.Enum {
				if vv := strings.TrimSpace(old); vv != "" {
					enumSet[vv] = struct{}{}
				}
			}
			for _, vv := range arg.Enum {
				if v := strings.TrimSpace(vv); v != "" {
					enumSet[v] = struct{}{}
				}
			}
			cur.Enum = sortedKeys(enumSet)
			a.argsByIdx[idx] = cur
		}
	}

	blockDirectiveSet := map[string]struct{}{}
	for _, name := range dslconfig.BlockDirectiveNames() {
		blockDirectiveSet[name] = struct{}{}
	}

	directiveNames := sortedKeysAgg(aggByName)
	spec := dslspec.Spec{
		Version:    strings.TrimSpace(existing.Version),
		Blocks:     make([]dslspec.BlockSpec, 0, len(blockSet)),
		Directives: make([]dslspec.DirectiveSpec, 0, len(directiveNames)),
	}
	if spec.Version == "" {
		spec.Version = "next-router/0.1"
	}

	for _, block := range orderBlocks(sortedKeys(blockSet)) {
		kind := "block"
		if block == "top" {
			kind = "top"
		}
		spec.Blocks = append(spec.Blocks, dslspec.BlockSpec{ID: block, Kind: kind})
	}

	for _, name := range directiveNames {
		a := aggByName[name]
		if _, ok := blockDirectiveSet[name]; ok {
			a.d.Kind = "block"
		}
		a.d.AllowedIn = orderBlocks(sortedKeys(a.blockSet))
		a.d.Modes = sortedKeys(a.modeSet)

		if len(a.argsByIdx) == 0 && len(a.d.Modes) > 0 {
			a.argsByIdx[0] = dslspec.DirectiveArgSpec{
				Name:     "mode",
				Type:     "enum",
				Required: true,
				Enum:     append([]string(nil), a.d.Modes...),
			}
		}
		if len(a.argsByIdx) > 0 {
			idxes := make([]int, 0, len(a.argsByIdx))
			for idx := range a.argsByIdx {
				idxes = append(idxes, idx)
			}
			slices.Sort(idxes)
			a.d.Args = make([]dslspec.DirectiveArgSpec, 0, len(idxes))
			for _, idx := range idxes {
				arg := a.argsByIdx[idx]
				arg.Name = strings.TrimSpace(arg.Name)
				if arg.Name == "" {
					arg.Name = "arg"
				}
				arg.Type = strings.TrimSpace(arg.Type)
				if arg.Type == "" {
					if len(arg.Enum) > 0 {
						arg.Type = "enum"
					} else {
						arg.Type = "value"
					}
				}
				if len(arg.Enum) > 0 {
					arg.Enum = sortedUnique(arg.Enum)
				}
				a.d.Args = append(a.d.Args, arg)
			}
		}
		spec.Directives = append(spec.Directives, a.d)
	}
	return spec
}

func buildLocale(spec dslspec.Spec, existing dslspec.LocaleBundle, zh bool) dslspec.LocaleBundle {
	out := dslspec.LocaleBundle{
		Locale:        strings.TrimSpace(existing.Locale),
		BlockTitles:   map[string]string{},
		DirectiveText: map[string]dslspec.DirectiveText{},
	}
	if out.Locale == "" {
		if zh {
			out.Locale = "zh-CN"
		} else {
			out.Locale = "en"
		}
	}

	existingBlockTitles := existing.BlockTitles
	if existingBlockTitles == nil {
		existingBlockTitles = map[string]string{}
	}
	for _, block := range spec.Blocks {
		id := block.ID
		title := strings.TrimSpace(existingBlockTitles[id])
		if title == "" {
			title = defaultBlockTitle(id, zh)
		}
		out.BlockTitles[id] = title
	}

	existingText := existing.DirectiveText
	if existingText == nil {
		existingText = map[string]dslspec.DirectiveText{}
	}
	metaSummary, ambiguousSummary := metadataSummaryByName()
	for _, d := range spec.Directives {
		id := d.ID
		txt := existingText[id]
		txt.Summary = strings.TrimSpace(txt.Summary)
		txt.Details = strings.TrimSpace(txt.Details)
		txt.Example = strings.TrimSpace(txt.Example)
		name := strings.TrimSpace(d.Name)
		if ambiguousSummary[name] {
			if zh {
				txt.Summary = "语义依赖所在 block。"
			} else {
				txt.Summary = "Semantics depend on block context."
			}
		} else if txt.Summary == "" {
			{
				txt.Summary = strings.TrimSpace(metaSummary[name])
			}
		}
		if txt.Summary == "" {
			if zh {
				txt.Summary = "待补充说明。"
			} else {
				txt.Summary = "Summary pending."
			}
		}
		out.DirectiveText[id] = txt
	}
	return out
}

func metadataSummaryByName() (map[string]string, map[string]bool) {
	out := map[string]string{}
	ambiguous := map[string]bool{}
	seen := map[string]map[string]struct{}{}
	for _, item := range dslconfig.DirectiveMetadataList() {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		summary := extractSummaryFromHover(item.Hover)
		if summary == "" {
			continue
		}
		if seen[name] == nil {
			seen[name] = map[string]struct{}{}
		}
		seen[name][summary] = struct{}{}
		if _, exists := out[name]; !exists {
			out[name] = summary
		}
	}
	for name, set := range seen {
		if len(set) > 1 {
			ambiguous[name] = true
		}
	}
	return out, ambiguous
}

func extractSummaryFromHover(hover string) string {
	h := strings.TrimSpace(hover)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, "\n\n", 2)
	if len(parts) == 2 {
		h = strings.TrimSpace(parts[1])
	}
	if idx := strings.IndexByte(h, '\n'); idx >= 0 {
		h = strings.TrimSpace(h[:idx])
	}
	return h
}

func defaultBlockTitle(id string, zh bool) string {
	if zh {
		switch id {
		case "top":
			return "文件级"
		case "provider":
			return "Provider"
		case "defaults":
			return "Defaults"
		case "match":
			return "Match"
		case "upstream_config":
			return "Upstream Config"
		case "auth":
			return "Auth"
		case "request":
			return "Request"
		case "upstream":
			return "Upstream"
		case "response":
			return "Response"
		case "error":
			return "Error"
		case "metrics":
			return "Metrics"
		case "balance":
			return "Balance"
		case "models":
			return "Models"
		default:
			return id
		}
	}
	switch id {
	case "top":
		return "File-level"
	case "provider":
		return "Provider"
	case "defaults":
		return "Defaults"
	case "match":
		return "Match"
	case "upstream_config":
		return "Upstream Config"
	case "auth":
		return "Auth"
	case "request":
		return "Request"
	case "upstream":
		return "Upstream"
	case "response":
		return "Response"
	case "error":
		return "Error"
	case "metrics":
		return "Metrics"
	case "balance":
		return "Balance"
	case "models":
		return "Models"
	default:
		return id
	}
}

func renderSpecYAML(spec dslspec.Spec) []byte {
	var b strings.Builder
	b.WriteString("version: ")
	b.WriteString(spec.Version)
	b.WriteString("\n")
	b.WriteString("blocks:\n")
	for _, block := range spec.Blocks {
		b.WriteString("  - id: ")
		b.WriteString(block.ID)
		b.WriteString("\n")
		b.WriteString("    kind: ")
		b.WriteString(block.Kind)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString("directives:\n")
	for _, d := range spec.Directives {
		b.WriteString("  - id: ")
		b.WriteString(d.ID)
		b.WriteString("\n")
		b.WriteString("    name: ")
		b.WriteString(d.Name)
		b.WriteString("\n")
		b.WriteString("    allowed_in: [")
		b.WriteString(strings.Join(d.AllowedIn, ", "))
		b.WriteString("]\n")
		b.WriteString("    kind: ")
		b.WriteString(d.Kind)
		b.WriteString("\n")
		b.WriteString("    repeatable: ")
		if d.Repeatable {
			b.WriteString("true\n")
		} else {
			b.WriteString("false\n")
		}
		if len(d.Modes) > 0 {
			b.WriteString("    modes: [")
			b.WriteString(strings.Join(d.Modes, ", "))
			b.WriteString("]\n")
		}
		if len(d.Args) > 0 {
			b.WriteString("    args:\n")
			for _, arg := range d.Args {
				b.WriteString("      - name: ")
				b.WriteString(arg.Name)
				b.WriteString("\n")
				b.WriteString("        type: ")
				b.WriteString(arg.Type)
				b.WriteString("\n")
				b.WriteString("        required: ")
				if arg.Required {
					b.WriteString("true\n")
				} else {
					b.WriteString("false\n")
				}
				if len(arg.Enum) > 0 {
					b.WriteString("        enum: [")
					b.WriteString(strings.Join(arg.Enum, ", "))
					b.WriteString("]\n")
				}
			}
		}
		if len(d.Constraints) > 0 {
			b.WriteString("    constraints: [")
			b.WriteString(strings.Join(d.Constraints, ", "))
			b.WriteString("]\n")
		}
	}
	return []byte(b.String())
}

func renderLocaleYAML(spec dslspec.Spec, bundle dslspec.LocaleBundle) []byte {
	var b strings.Builder
	b.WriteString("locale: ")
	b.WriteString(bundle.Locale)
	b.WriteString("\n\n")

	b.WriteString("block_titles:\n")
	for _, block := range spec.Blocks {
		b.WriteString("  ")
		b.WriteString(block.ID)
		b.WriteString(": ")
		b.WriteString(quoteYAMLScalar(bundle.BlockTitles[block.ID]))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString("directive_text:\n")
	for _, d := range spec.Directives {
		txt := bundle.DirectiveText[d.ID]
		b.WriteString("  ")
		b.WriteString(d.ID)
		b.WriteString(":\n")
		b.WriteString("    summary: ")
		b.WriteString(quoteYAMLScalar(txt.Summary))
		b.WriteString("\n")
		if txt.Details != "" {
			b.WriteString("    details: ")
			b.WriteString(quoteYAMLScalar(txt.Details))
			b.WriteString("\n")
		}
		if txt.Example != "" {
			b.WriteString("    example: ")
			b.WriteString(quoteYAMLScalar(txt.Example))
			b.WriteString("\n")
		}
	}
	return []byte(b.String())
}

func quoteYAMLScalar(s string) string {
	v := strings.TrimSpace(s)
	if v == "" {
		return `""`
	}
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func sortedKeysAgg(m map[string]*directiveAgg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func sortedUnique(in []string) []string {
	set := map[string]struct{}{}
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		set[v] = struct{}{}
	}
	return sortedKeys(set)
}

func normalizeBlock(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	if v == "_top" {
		return "top"
	}
	return v
}

func orderBlocks(blocks []string) []string {
	order := map[string]int{
		"top":             0,
		"provider":        1,
		"defaults":        2,
		"match":           3,
		"upstream_config": 4,
		"auth":            5,
		"request":         6,
		"upstream":        7,
		"response":        8,
		"error":           9,
		"metrics":         10,
		"balance":         11,
		"models":          12,
	}
	out := append([]string(nil), blocks...)
	slices.SortFunc(out, func(a, b string) int {
		wa, oka := order[a]
		wb, okb := order[b]
		switch {
		case oka && okb:
			return cmp.Compare(wa, wb)
		case oka:
			return -1
		case okb:
			return 1
		default:
			return cmp.Compare(a, b)
		}
	})
	return out
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "onr-dslspec-sync: "+format+"\n", args...)
	os.Exit(1)
}
