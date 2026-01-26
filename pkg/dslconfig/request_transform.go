package dslconfig

import (
	"strings"

	"github.com/edgefn/open-next-router/pkg/dslmeta"
)

type ModelMapConfig struct {
	// Map holds model -> expr mapping; the expr typically evaluates to a string.
	Map map[string]string
	// DefaultExpr is applied when Map does not contain the model; optional.
	DefaultExpr string
}

type RequestTransform struct {
	ModelMap ModelMapConfig
	JSONOps  []JSONOp
}

type ProviderRequestTransform struct {
	Defaults RequestTransform
	Matches  []MatchRequestTransform
}

type MatchRequestTransform struct {
	API    string
	Stream *bool

	Transform RequestTransform
}

func (p ProviderRequestTransform) Select(meta *dslmeta.Meta) (RequestTransform, bool) {
	if meta == nil {
		return RequestTransform{}, false
	}
	api := strings.TrimSpace(meta.API)
	if api == "" {
		return RequestTransform{}, false
	}
	out := p.Defaults
	if m, ok := p.selectMatch(api, meta.IsStream); ok {
		out = mergeRequestTransform(out, m.Transform)
	}
	if out.ModelMap.Map == nil && strings.TrimSpace(out.ModelMap.DefaultExpr) == "" && len(out.JSONOps) == 0 {
		return RequestTransform{}, false
	}
	return out, true
}

func (p ProviderRequestTransform) selectMatch(api string, stream bool) (MatchRequestTransform, bool) {
	for _, m := range p.Matches {
		if m.API != "" && m.API != api {
			continue
		}
		if m.Stream != nil && *m.Stream != stream {
			continue
		}
		return m, true
	}
	return MatchRequestTransform{}, false
}

func mergeRequestTransform(base, override RequestTransform) RequestTransform {
	out := base
	if len(override.ModelMap.Map) > 0 {
		if out.ModelMap.Map == nil {
			out.ModelMap.Map = map[string]string{}
		}
		for k, v := range override.ModelMap.Map {
			out.ModelMap.Map[k] = v
		}
	}
	if strings.TrimSpace(override.ModelMap.DefaultExpr) != "" {
		out.ModelMap.DefaultExpr = override.ModelMap.DefaultExpr
	}
	if len(override.JSONOps) > 0 {
		out.JSONOps = append(out.JSONOps, override.JSONOps...)
	}
	return out
}

func (t RequestTransform) Apply(meta *dslmeta.Meta) {
	if meta == nil {
		return
	}
	if meta.DSLModelMapped == "" {
		meta.DSLModelMapped = meta.ActualModelName
	}
	m := t.ModelMap
	if len(m.Map) == 0 && strings.TrimSpace(m.DefaultExpr) == "" {
		return
	}

	key := strings.TrimSpace(meta.ActualModelName)
	if key == "" {
		return
	}
	if expr, ok := m.Map[key]; ok && strings.TrimSpace(expr) != "" {
		meta.DSLModelMapped = evalStringExpr(expr, meta)
		return
	}
	if strings.TrimSpace(m.DefaultExpr) != "" {
		meta.DSLModelMapped = evalStringExpr(m.DefaultExpr, meta)
	}
}

type JSONOp struct {
	Op string

	Path      string
	FromPath  string
	ToPath    string
	ValueExpr string
}
