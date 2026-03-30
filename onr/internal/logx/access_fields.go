package logx

import "sort"

type AccessLogContextFieldSpec struct {
	CtxKey string
	LogKey string
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

var standardUsageFieldOrder = []string{
	"input_tokens",
	"output_tokens",
	"total_tokens",
	"cache_read_tokens",
	"cache_write_tokens",
}

var accessLogContextFieldSpecs = []AccessLogContextFieldSpec{
	{CtxKey: "onr.provider", LogKey: "provider"},
	{CtxKey: "onr.provider_source", LogKey: "provider_source"},
	{CtxKey: "onr.api", LogKey: "api"},
	{CtxKey: "onr.stream", LogKey: "stream"},
	{CtxKey: "onr.model", LogKey: "model"},
	{CtxKey: "onr.usage_stage", LogKey: "usage_stage"},
	{CtxKey: "onr.usage_input_tokens", LogKey: "input_tokens"},
	{CtxKey: "onr.usage_output_tokens", LogKey: "output_tokens"},
	{CtxKey: "onr.usage_total_tokens", LogKey: "total_tokens"},
	{CtxKey: "onr.usage_cache_read_tokens", LogKey: "cache_read_tokens"},
	{CtxKey: "onr.usage_cache_write_tokens", LogKey: "cache_write_tokens"},
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
	{CtxKey: "onr.upstream_status", LogKey: "upstream_status"},
	{CtxKey: "onr.finish_reason", LogKey: "finish_reason"},
	{CtxKey: "onr.ttft_ms", LogKey: "ttft_ms"},
	{CtxKey: "onr.tps", LogKey: "tps"},
}

var tokenFieldKeys = newFieldSet(trailingTokenFieldOrder)
var standardUsageFieldKeys = newFieldSet(standardUsageFieldOrder)
var fixedAccessFieldKeys = newFixedAccessFieldSet(accessLogContextFieldSpecs, tokenFieldKeys)
var allowedAccessLogVars = newAccessLogAllowedVarSet(accessLogContextFieldSpecs)

func AccessLogContextFieldSpecs() []AccessLogContextFieldSpec {
	out := make([]AccessLogContextFieldSpec, len(accessLogContextFieldSpecs))
	copy(out, accessLogContextFieldSpecs)
	return out
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
