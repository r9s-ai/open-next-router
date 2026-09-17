// Package jsext implements host-independent, synchronous JavaScript hooks.
package jsext

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
)

type Stage string

const (
	Request         Stage = "request"
	AfterRequestMap Stage = "after_req_map"
	ResponseHeaders Stage = "response_headers"
	Response        Stage = "response"
	SSEEvent        Stage = "sse_event"
	Log             Stage = "log"
)

var stages = [...]Stage{Request, AfterRequestMap, ResponseHeaders, Response, SSEEvent, Log}

type HandlerSource struct {
	Body         string
	File         string
	Location     string
	Line, Column int
	Off          bool
	program      *goja.Program
}

type Limits struct {
	Timeout       time.Duration
	StreamTimeout time.Duration
	BodyBytes     int64
	EventBytes    int64
}

func DefaultLimits() Limits {
	return Limits{200 * time.Millisecond, 10 * time.Millisecond, 8 << 20, 1 << 20}
}
func (l Limits) Merge(v Limits) Limits {
	if v.Timeout != 0 {
		l.Timeout = v.Timeout
	}
	if v.StreamTimeout != 0 {
		l.StreamTimeout = v.StreamTimeout
	}
	if v.BodyBytes != 0 {
		l.BodyBytes = v.BodyBytes
	}
	if v.EventBytes != 0 {
		l.EventBytes = v.EventBytes
	}
	return l
}

type Config struct {
	Handlers map[Stage]HandlerSource
	Limits   Limits
}

func (c Config) Merge(v Config) Config {
	out := Config{Limits: c.Limits.Merge(v.Limits)}
	if len(c.Handlers)+len(v.Handlers) > 0 {
		out.Handlers = map[Stage]HandlerSource{}
	}
	for k, h := range c.Handlers {
		out.Handlers[k] = h
	}
	for k, h := range v.Handlers {
		out.Handlers[k] = h
	}
	return out
}
func (c Config) Has(stage Stage) bool { h, ok := c.Handlers[stage]; return ok && !h.Off }
func (c Config) Enabled() bool {
	for _, s := range stages {
		if c.Has(s) {
			return true
		}
	}
	return false
}

// CompiledHandlers is the effective slot set selected from a prepared Provider.
// It is immutable and may be shared by concurrent, independent sessions.
type CompiledHandlers struct{ Config }

type Match struct {
	API    string
	Stream *bool
	Config Config
}
type Provider struct {
	Defaults Config
	Matches  []Match
	HTTP     HTTPPolicy
}

func (p Provider) Select(api string, stream bool) CompiledHandlers {
	c := Config{Limits: DefaultLimits()}.Merge(p.Defaults)
	for _, m := range p.Matches {
		if (m.API == "" || m.API == api) && (m.Stream == nil || *m.Stream == stream) {
			c = c.Merge(m.Config)
			break
		}
	}
	return CompiledHandlers{Config: c}
}

type RuntimeConfig struct {
	Root   string                `yaml:"root"`
	Reload string                `yaml:"reload"`
	HTTP   map[string]HTTPPolicy `yaml:"http"`
}

func (c RuntimeConfig) Validate() error {
	switch c.Reload {
	case "", "off", "watch", "poll":
	default:
		return fmt.Errorf("js.reload must be off, watch or poll")
	}
	for name, p := range c.HTTP {
		if err := p.validate(); err != nil {
			return fmt.Errorf("js.http.%s: %w", name, err)
		}
	}
	return nil
}

// Clone detaches host authorization from mutable YAML configuration maps.
func (c RuntimeConfig) Clone() RuntimeConfig {
	out := c
	out.HTTP = make(map[string]HTTPPolicy, len(c.HTTP))
	for name, p := range c.HTTP {
		out.HTTP[name] = p.clone()
	}
	return out
}

// Compiler belongs to one prepare transaction. File bytes and compiled programs
// are reused across providers and stages; it is discarded after publication.
type Compiler struct {
	host     RuntimeConfig
	files    map[string]string
	programs map[string]*goja.Program
}

func NewCompiler(host RuntimeConfig) *Compiler {
	return &Compiler{host: host.Clone(), files: map[string]string{}, programs: map[string]*goja.Program{}}
}

// Prepare compiles all handlers before publication, including overridden defaults.
// No file access occurs in a request. The returned provider owns a policy copy.
func (p Provider) Prepare(host RuntimeConfig, name string) (Provider, error) {
	return NewCompiler(host).Prepare(p, name)
}
func (c *Compiler) Prepare(p Provider, name string) (Provider, error) {
	p.HTTP = c.host.HTTP[name].clone()
	var err error
	p.Defaults, err = c.prepareConfig(p.Defaults)
	if err != nil {
		return Provider{}, err
	}
	p.Matches = append([]Match(nil), p.Matches...)
	for i := range p.Matches {
		p.Matches[i].Config, err = c.prepareConfig(p.Matches[i].Config)
		if err != nil {
			return Provider{}, err
		}
	}
	return p, nil
}
func (c *Compiler) prepareConfig(cfg Config) (Config, error) {
	cfg = cfg.Merge(Config{})
	for stage, h := range cfg.Handlers {
		if h.Off {
			continue
		}
		if h.File != "" {
			body, ok := c.files[h.File]
			if !ok {
				var err error
				body, err = readScript(c.host.Root, h.File)
				if err != nil {
					return Config{}, fmt.Errorf("%s: %w", h.Location, err)
				}
				c.files[h.File] = body
			}
			h.Body = body
			h.Location = filepath.Join(c.host.Root, h.File)
			h.Line = 1
			h.Column = 1
		}
		key := fmt.Sprintf("%s:%d:%d\x00%s", h.Location, h.Line, h.Column, h.Body)
		program, ok := c.programs[key]
		if !ok {
			var err error
			program, err = CompileAt(h.Location, h.Body, h.Line, h.Column)
			if err != nil {
				return Config{}, err
			}
			c.programs[key] = program
		}
		h.program = program
		cfg.Handlers[stage] = h
	}
	return cfg, nil
}
func readScript(root, name string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("js.root is required for file handlers")
	}
	if filepath.IsAbs(name) || !filepath.IsLocal(name) || strings.Contains(name, "://") {
		return "", fmt.Errorf("script must be a local relative path")
	}
	// os.Root provides traversal-resistant opens, including symlink resolution.
	r, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()
	f, err := r.Open(name)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, 8<<20+1))
	if err != nil {
		return "", err
	}
	if len(b) > 8<<20 {
		return "", fmt.Errorf("script exceeds 8 MiB")
	}
	return string(b), nil
}

func Compile(location, body string) (*goja.Program, error) { return CompileAt(location, body, 1, 1) }
func CompileAt(location, body string, line, column int) (*goja.Program, error) {
	source := "(function(ctx){\"use strict\";\n" + body + "\n})"
	tree, err := parser.ParseFile(nil, location, source, 0)
	if err != nil {
		return nil, sourceError(location, body, line, column, err)
	}
	if containsAsync(reflect.ValueOf(tree)) {
		return nil, fmt.Errorf("%s:%d:%d: asynchronous and generator functions are not supported", location, max(line, 1), max(column, 1))
	}
	program, err := goja.CompileAST(tree, true)
	if err != nil {
		return nil, sourceError(location, body, line, column, err)
	}
	return program, nil
}
func containsAsync(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		return containsAsync(v.Elem())
	}
	if v.CanInterface() {
		switch n := v.Interface().(type) {
		case ast.FunctionLiteral:
			if n.Async || n.Generator {
				return true
			}
		case ast.ArrowFunctionLiteral:
			if n.Async {
				return true
			}
		case ast.AwaitExpression:
			return true
		}
	}
	switch v.Kind() {
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if containsAsync(v.Field(i)) {
				return true
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if containsAsync(v.Index(i)) {
				return true
			}
		}
	}
	return false
}
