package logx

import "sort"

type AccessLogContextFieldSpec struct {
	CtxKey string
	LogKey string
}

var standardUsageContextFieldSpecs = []AccessLogContextFieldSpec{
	{CtxKey: "onr.usage_input_tokens", LogKey: "input_tokens"},
	{CtxKey: "onr.usage_output_tokens", LogKey: "output_tokens"},
	{CtxKey: "onr.usage_total_tokens", LogKey: "total_tokens"},
	{CtxKey: "onr.usage_cache_read_tokens", LogKey: "cache_read_tokens"},
	{CtxKey: "onr.usage_cache_write_tokens", LogKey: "cache_write_tokens"},
}

var costContextFieldSpecs = []AccessLogContextFieldSpec{
	{CtxKey: "onr.cost_total", LogKey: "cost_total"},
	{CtxKey: "onr.cost_input", LogKey: "cost_input"},
	{CtxKey: "onr.cost_output", LogKey: "cost_output"},
	{CtxKey: "onr.cost_cache_read", LogKey: "cost_cache_read"},
	{CtxKey: "onr.cost_cache_write", LogKey: "cost_cache_write"},
	{CtxKey: "onr.billable_input_tokens", LogKey: "billable_input_tokens"},
	{CtxKey: "onr.cost_multiplier", LogKey: "cost_multiplier"},
	{CtxKey: "onr.cost_model", LogKey: "cost_model"},
	{CtxKey: "onr.cost_channel", LogKey: "cost_channel"},
	{CtxKey: "onr.cost_unit", LogKey: "cost_unit"},
}

var trailingTokenFieldOrder = []string{
	"input_tokens",
	"output_tokens",
	"total_tokens",
	"cache_read_tokens",
	"cache_write_tokens",
	"billable_input_tokens",
	"cost_input",
	"cost_output",
	"cost_cache_read",
	"cost_cache_write",
	"cost_total",
}

var standardUsageFieldOrder = logKeysOfSpecs(standardUsageContextFieldSpecs)

var baseAccessLogContextFieldSpecs = []AccessLogContextFieldSpec{
	{CtxKey: "onr.provider", LogKey: "provider"},
	{CtxKey: "onr.provider_source", LogKey: "provider_source"},
	{CtxKey: "onr.api", LogKey: "api"},
	{CtxKey: "onr.stream", LogKey: "stream"},
	{CtxKey: "onr.model", LogKey: "model"},
	{CtxKey: "onr.usage_stage", LogKey: "usage_stage"},
	{CtxKey: "onr.upstream_status", LogKey: "upstream_status"},
	{CtxKey: "onr.finish_reason", LogKey: "finish_reason"},
	{CtxKey: "onr.ttft_ms", LogKey: "ttft_ms"},
	{CtxKey: "onr.tps", LogKey: "tps"},
}

var accessLogContextFieldSpecs = combineAccessLogContextFieldSpecs(
	baseAccessLogContextFieldSpecs,
	standardUsageContextFieldSpecs,
	costContextFieldSpecs,
)

var tokenFieldKeys = newFieldSet(trailingTokenFieldOrder)
var standardUsageFieldKeys = newFieldSet(standardUsageFieldOrder)
var standardUsageContextKeys = newContextKeySet(standardUsageContextFieldSpecs)
var costContextKeys = newContextKeySet(costContextFieldSpecs)
var fixedAccessFieldKeys = newFixedAccessFieldSet(accessLogContextFieldSpecs, tokenFieldKeys)
var allowedAccessLogVars = newAccessLogAllowedVarSet(accessLogContextFieldSpecs)

func AccessLogContextFieldSpecs() []AccessLogContextFieldSpec {
	out := make([]AccessLogContextFieldSpec, len(accessLogContextFieldSpecs))
	copy(out, accessLogContextFieldSpecs)
	return out
}

func StandardUsageContextFieldSpecs() []AccessLogContextFieldSpec {
	out := make([]AccessLogContextFieldSpec, len(standardUsageContextFieldSpecs))
	copy(out, standardUsageContextFieldSpecs)
	return out
}

func CostContextFieldSpecs() []AccessLogContextFieldSpec {
	out := make([]AccessLogContextFieldSpec, len(costContextFieldSpecs))
	copy(out, costContextFieldSpecs)
	return out
}

func StandardUsageContextKey(logKey string) (string, bool) {
	ctxKey, ok := standardUsageContextKeys[logKey]
	return ctxKey, ok
}

func CostContextKey(logKey string) (string, bool) {
	ctxKey, ok := costContextKeys[logKey]
	return ctxKey, ok
}

func IsStandardUsageField(key string) bool {
	_, ok := standardUsageFieldKeys[key]
	return ok
}

func AccessLogAllowedVars() []string {
	keys := make([]string, 0, len(allowedAccessLogVars))
	for k := range allowedAccessLogVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func newFieldSet(keys []string) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		out[key] = struct{}{}
	}
	return out
}

func logKeysOfSpecs(specs []AccessLogContextFieldSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.LogKey)
	}
	return out
}

func combineAccessLogContextFieldSpecs(groups ...[]AccessLogContextFieldSpec) []AccessLogContextFieldSpec {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	out := make([]AccessLogContextFieldSpec, 0, total)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}

func newContextKeySet(specs []AccessLogContextFieldSpec) map[string]string {
	out := make(map[string]string, len(specs))
	for _, spec := range specs {
		out[spec.LogKey] = spec.CtxKey
	}
	return out
}

func newFixedAccessFieldSet(specs []AccessLogContextFieldSpec, trailing map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if _, ok := trailing[spec.LogKey]; ok {
			continue
		}
		out[spec.LogKey] = struct{}{}
	}
	return out
}

func newAccessLogAllowedVarSet(specs []AccessLogContextFieldSpec) map[string]struct{} {
	out := map[string]struct{}{
		"time_local": {},
		"status":     {},
		"latency":    {},
		"latency_ms": {},
		"client_ip":  {},
		"method":     {},
		"path":       {},
		"request_id": {},
		"appname":    {},
	}
	for _, spec := range specs {
		out[spec.LogKey] = struct{}{}
	}
	return out
}
