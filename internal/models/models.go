package models

import (
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Strategy string

const (
	StrategyRoundRobin Strategy = "round_robin"
)

type Route struct {
	Providers []string `yaml:"providers"`
	Strategy  Strategy `yaml:"strategy"`
	OwnedBy   string   `yaml:"owned_by"`
}

type File struct {
	Models map[string]Route `yaml:"models"`
}

// Router holds model -> providers routing and per-model round-robin state.
type Router struct {
	mu      sync.Mutex
	routes  map[string]Route
	nextIdx map[string]int
}

func NewRouter(routes map[string]Route) *Router {
	out := &Router{
		routes:  map[string]Route{},
		nextIdx: map[string]int{},
	}
	for id, r := range routes {
		mid := normalizeModelID(id)
		if mid == "" {
			continue
		}
		out.routes[mid] = normalizeRoute(r)
	}
	return out
}

func (r *Router) Models() []string {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.routes))
	for id := range r.routes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *Router) NextProvider(modelID string) (string, bool) {
	if r == nil {
		return "", false
	}
	id := normalizeModelID(modelID)
	if id == "" {
		return "", false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rt, ok := r.routes[id]
	if !ok || len(rt.Providers) == 0 {
		return "", false
	}
	strat := rt.Strategy
	if strat == "" {
		strat = StrategyRoundRobin
	}
	switch strat {
	case StrategyRoundRobin:
		i := r.nextIdx[id] % len(rt.Providers)
		r.nextIdx[id] = (i + 1) % len(rt.Providers)
		return rt.Providers[i], true
	default:
		// Unknown strategy: fall back to round-robin for forward compatibility.
		i := r.nextIdx[id] % len(rt.Providers)
		r.nextIdx[id] = (i + 1) % len(rt.Providers)
		return rt.Providers[i], true
	}
}

func (r *Router) ToOpenAIList() map[string]any {
	now := time.Now().Unix()
	data := make([]any, 0)
	for _, id := range r.Models() {
		ownedBy := "open-next-router"
		r.mu.Lock()
		if rt, ok := r.routes[id]; ok && strings.TrimSpace(rt.OwnedBy) != "" {
			ownedBy = strings.TrimSpace(rt.OwnedBy)
		}
		r.mu.Unlock()
		data = append(data, map[string]any{
			"id":       id,
			"object":   "model",
			"created":  now,
			"owned_by": ownedBy,
		})
	}
	return map[string]any{
		"object": "list",
		"data":   data,
	}
}

// Load reads a model routing file. If the file does not exist, returns an empty router and nil error.
func Load(path string) (*Router, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return NewRouter(nil), nil
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewRouter(nil), nil
		}
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return NewRouter(f.Models), nil
}

func normalizeModelID(s string) string {
	return strings.TrimSpace(s)
}

func normalizeRoute(r Route) Route {
	out := r
	out.OwnedBy = strings.TrimSpace(out.OwnedBy)
	if out.Strategy == "" {
		out.Strategy = StrategyRoundRobin
	}
	provs := make([]string, 0, len(out.Providers))
	for _, p := range out.Providers {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		provs = append(provs, p)
	}
	out.Providers = provs
	return out
}
