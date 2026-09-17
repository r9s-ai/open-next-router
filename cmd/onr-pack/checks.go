package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslconfig"
	"github.com/r9s-ai/open-next-router/onr-core/pkg/dslmeta"
)

const (
	checkRequiredUsage = "required-usage"
	checkAll           = "all"
)

type checkList []string

func (c *checkList) Set(value string) error {
	for _, item := range strings.Split(value, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if name == "" {
			continue
		}
		*c = append(*c, name)
	}
	return nil
}

func (c *checkList) String() string {
	if c == nil || len(*c) == 0 {
		return ""
	}
	return strings.Join(*c, ",")
}

func runExtraChecks(sourcePath string, checks checkList) error {
	names, err := normalizeCheckNames(checks)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	providers, err := loadProviderFiles(sourcePath)
	if err != nil {
		return err
	}
	var failures []string
	for _, name := range names {
		switch name {
		case checkRequiredUsage:
			failures = append(failures, checkRequiredUsageConfig(providers)...)
		default:
			return fmt.Errorf("unknown check %q", name)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("%s", formatCheckFailures(failures))
}

func normalizeCheckNames(checks checkList) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, name := range checks {
		switch name {
		case checkAll:
			if _, ok := seen[checkRequiredUsage]; !ok {
				seen[checkRequiredUsage] = struct{}{}
				out = append(out, checkRequiredUsage)
			}
		case checkRequiredUsage:
			if _, ok := seen[name]; !ok {
				seen[name] = struct{}{}
				out = append(out, name)
			}
		default:
			return nil, fmt.Errorf("unknown check %q (known: %s, %s)", name, checkRequiredUsage, checkAll)
		}
	}
	return out, nil
}

func loadProviderFiles(sourcePath string) ([]dslconfig.ProviderFile, error) {
	return dslconfig.InspectProvidersPath(sourcePath)
}

func checkRequiredUsageConfig(providers []dslconfig.ProviderFile) []string {
	var failures []string
	for _, provider := range providers {
		for _, match := range provider.Routing.Matches {
			if strings.TrimSpace(match.API) != "chat.completions" {
				continue
			}
			if !looksLikeChatCompletionsPath(match.SetPath) {
				continue
			}
			if match.Stream == nil {
				missingNonStream := !hasUsageExtractFor(provider, false)
				missingStream := !hasUsageExtractFor(provider, true)
				switch {
				case missingNonStream && missingStream:
					failures = append(failures, requiredUsageFailure(provider, match, "any"))
				case missingNonStream:
					failures = append(failures, requiredUsageFailure(provider, match, "false"))
				case missingStream:
					failures = append(failures, requiredUsageFailure(provider, match, "true"))
				}
				continue
			}
			if !hasUsageExtractFor(provider, *match.Stream) {
				failures = append(failures, requiredUsageFailure(provider, match, fmt.Sprintf("%v", *match.Stream)))
			}
		}
	}
	sort.Strings(failures)
	return failures
}

func looksLikeChatCompletionsPath(expr string) bool {
	v := strings.ToLower(strings.TrimSpace(expr))
	return strings.Contains(v, "chat/completions")
}

func hasUsageExtractFor(provider dslconfig.ProviderFile, stream bool) bool {
	_, ok := provider.Usage.Select(&dslmeta.Meta{API: "chat.completions", IsStream: stream})
	return ok
}

func requiredUsageFailure(provider dslconfig.ProviderFile, match dslconfig.RoutingMatch, stream string) string {
	return fmt.Sprintf(
		"required-usage\nprovider: %s\nfile: %s\nmatch: api=%q stream=%v\nupstream: %s\nfix: add metrics { usage_extract ... } for this match",
		provider.Name,
		displayProviderPath(provider.Path),
		match.API,
		stream,
		displayStringExpr(match.SetPath),
	)
}

func displayProviderPath(path string) string {
	if rel, err := filepath.Rel(".", path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func displayStringExpr(expr string) string {
	raw := strings.TrimSpace(expr)
	if v, err := strconv.Unquote(raw); err == nil {
		return v
	}
	return raw
}

func formatCheckFailures(failures []string) string {
	lines := make([]string, 0, len(failures))
	for _, failure := range failures {
		lines = append(lines, "  - "+strings.ReplaceAll(failure, "\n", "\n    "))
	}
	return strings.Join(lines, "\n")
}
