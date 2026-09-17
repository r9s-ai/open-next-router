package jsext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/dop251/goja"
)

type Meta struct {
	RequestID     string `json:"requestId"`
	Attempt       int    `json:"attempt"`
	Provider      string `json:"provider"`
	API           string `json:"api"`
	OriginalModel string `json:"originalModel"`
	UpstreamModel string `json:"upstreamModel"`
	Stream        bool   `json:"stream"`
}
type Message struct {
	Body                      string
	Headers                   http.Header
	Method, Path, ContentType string
	Status                    int
}
type Outcome struct {
	Outcome      string         `json:"outcome"`
	Status       int            `json:"status"`
	Usage        map[string]any `json:"usage,omitempty"`
	FinishReason string         `json:"finishReason,omitempty"`
}
type Rejection struct {
	Status int
	Body   []byte
}

func (e *Rejection) Error() string { return fmt.Sprintf("JavaScript rejected request (%d)", e.Status) }

type Error struct {
	Stage Stage
	Cause error
}

func (e *Error) Error() string { return fmt.Sprintf("JavaScript %s: %v", e.Stage, e.Cause) }
func (e *Error) Unwrap() error { return e.Cause }

// Terminal reports errors which a host must never retry on another channel.
func Terminal(err error) bool {
	var e *Error
	var r *Rejection
	return errors.As(err, &e) || errors.As(err, &r)
}

type Logger func(level, message string, fields map[string]any)

// Session belongs to exactly one attempt and is not safe for concurrent calls.
// Compiled programs are shared; VM, state and HTTP budgets never are.
type Session struct {
	config            CompiledHandlers
	policy            HTTPPolicy
	vm                *goja.Runtime
	object            *goja.Object
	freeze            goja.Callable
	preventExtensions goja.Callable
	parseJSON         goja.Callable
	stringifyJSON     goja.Callable
	views             map[string]goja.Value
	stage             Stage
	hookContext       context.Context
	httpCalls         int
	rejected          *Rejection
	finished          bool
	logger            Logger
	request           *Message
	response          *Message
}

// NewSession requires a prepared configuration selected for this attempt. The
// host must treat configuration and policy as immutable. A nil logger discards logs.
func NewSession(config CompiledHandlers, meta Meta, policy HTTPPolicy, logger Logger) (*Session, error) {
	if !config.Enabled() {
		return nil, nil
	}
	for _, stage := range stages {
		if config.Has(stage) && config.Handlers[stage].program == nil {
			return nil, fmt.Errorf("%s handler was not prepared", stage)
		}
	}
	s := &Session{config: config, policy: policy.clone(), vm: goja.New(), logger: logger}
	s.vm.SetMaxCallStackSize(256)
	// Disable dynamic compilation as well as asynchronous entry points.
	_, err := s.vm.RunString(`Object.defineProperty(Function.prototype,"constructor",{value:undefined}); globalThis.eval=undefined; globalThis.Function=undefined; globalThis.Promise=undefined;`)
	if err != nil {
		return nil, err
	}
	v, err := s.vm.RunString(`(function(keys, freezeObject){return function freeze(x){if(x&&typeof x==="object"){const names=keys(x);for(let i=0;i<names.length;i++)freeze(x[names[i]]);freezeObject(x);}return x;};})(Object.keys,Object.freeze)`)
	if err != nil {
		return nil, err
	}
	s.freeze, _ = goja.AssertFunction(v)
	s.preventExtensions, _ = goja.AssertFunction(s.vm.Get("Object").ToObject(s.vm).Get("preventExtensions"))
	s.parseJSON, _ = goja.AssertFunction(s.vm.Get("JSON").ToObject(s.vm).Get("parse"))
	s.stringifyJSON, _ = goja.AssertFunction(s.vm.Get("JSON").ToObject(s.vm).Get("stringify"))
	s.views = map[string]goja.Value{}
	s.object = s.vm.NewObject()
	for _, name := range []string{"meta", "request", "response", "event", "result", "http", "log"} {
		_ = s.object.DefineAccessorProperty(name, s.vm.ToValue(func(goja.FunctionCall) goja.Value {
			if v, ok := s.views[name]; ok {
				return v
			}
			return goja.Undefined()
		}), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
	if err = s.readonly("meta", s.jsonValue(meta)); err != nil {
		return nil, err
	}
	_ = s.object.DefineDataProperty("state", s.vm.NewObject(), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	httpObj := s.vm.NewObject()
	_ = httpObj.Set("request", func(call goja.FunctionCall) goja.Value {
		var r httpRequest
		if err := s.decode(call.Argument(0), &r); err != nil {
			panic(s.vm.NewGoError(err))
		}
		response, err := s.httpRequest(r)
		if err != nil {
			panic(s.vm.NewGoError(err))
		}
		return s.jsonValue(response)
	})
	if err := s.readonly("http", httpObj); err != nil {
		return nil, err
	}
	_ = s.object.DefineDataProperty("reject", s.vm.ToValue(func(call goja.FunctionCall) goja.Value {
		if s.stage != Request && s.stage != AfterRequestMap && s.stage != ResponseHeaders && s.stage != Response {
			panic(s.vm.NewTypeError("reject is not allowed in this stage"))
		}
		value := call.Argument(0)
		numeric := false
		switch value.Export().(type) {
		case int64, float64:
			numeric = true
		}
		n := value.ToInteger()
		if !numeric || n < 400 || n > 599 || call.Argument(0).ToFloat() != float64(n) {
			panic(s.vm.NewTypeError("reject status must be 400..599"))
		}
		raw, err := s.stringifyJSON(goja.Undefined(), call.Argument(1))
		if err != nil || goja.IsUndefined(raw) {
			panic(s.vm.NewTypeError("rejection body must be JSON serializable"))
		}
		b := []byte(raw.String())
		if int64(len(b)) > s.config.Limits.BodyBytes {
			panic(s.vm.NewTypeError("invalid rejection body"))
		}
		s.rejected = &Rejection{Status: int(n), Body: b}
		s.vm.Interrupt(s.rejected)
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	log := s.vm.NewObject()
	for _, level := range []string{"debug", "info", "warn", "error"} {
		_ = log.Set(level, func(call goja.FunctionCall) goja.Value {
			var fields map[string]any
			if !goja.IsUndefined(call.Argument(1)) {
				if err := s.decode(call.Argument(1), &fields); err != nil {
					panic(s.vm.NewGoError(err))
				}
			}
			if s.logger != nil {
				s.logger(level, call.Argument(0).String(), fields)
			}
			return goja.Undefined()
		})
	}
	if err := s.readonly("log", log); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Session) Has(stage Stage) bool { return s != nil && s.config.Has(stage) }
func (s *Session) Limits() Limits {
	if s == nil {
		return DefaultLimits()
	}
	return s.config.Limits
}
func (s *Session) jsonValue(v any) goja.Value {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	value, err := s.parseJSON(goja.Undefined(), s.vm.ToValue(string(b)))
	if err != nil {
		panic(err)
	}
	return value
}
func (s *Session) decode(v goja.Value, out any) error {
	raw, err := s.stringifyJSON(goja.Undefined(), v)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw.String()), out)
}
func (s *Session) readonly(name string, v goja.Value) error {
	if _, err := s.freeze(goja.Undefined(), v); err != nil {
		return err
	}
	s.views[name] = v
	return nil
}
func (s *Session) message(name string, m *Message, bodyMutable, headersMutable bool) error {
	obj := s.jsonValue(map[string]any{"body": m.Body, "headers": visibleHeaders(m.Headers), "method": m.Method, "path": m.Path, "contentType": m.ContentType, "status": m.Status}).ToObject(s.vm)
	for _, key := range []string{"body", "headers", "method", "path", "contentType", "status"} {
		mutable := key == "body" && bodyMutable || key == "headers" && headersMutable
		if !mutable {
			v := obj.Get(key)
			if _, err := s.freeze(goja.Undefined(), v); err != nil {
				return err
			}
			if err := obj.DefineDataProperty(key, v, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
				return err
			}
		}
	}
	if _, err := s.preventExtensions(goja.Undefined(), obj); err != nil {
		return err
	}
	s.views[name] = obj
	return nil
}

// Run requires a non-nil context and message for a request or response stage.
// A nil Session is the disabled fast path. Run applies validated changes transactionally. Failed hooks cannot leak
// partially rewritten headers or bodies into the host request.
func (s *Session) Run(ctx context.Context, stage Stage, m *Message) error {
	if s == nil {
		return nil
	}
	if stage == Request || stage == AfterRequestMap {
		s.request = cloneMessage(m)
	} else {
		s.response = cloneMessage(m)
	}
	if !s.Has(stage) {
		return nil
	}
	bodyMutable := stage == Request || stage == AfterRequestMap || stage == Response
	if bodyMutable {
		if !strings.Contains(strings.ToLower(m.ContentType), "json") || !json.Valid([]byte(m.Body)) || !utf8.ValidString(m.Body) {
			return &Error{stage, fmt.Errorf("body hooks require JSON")}
		}
		if int64(len(m.Body)) > s.config.Limits.BodyBytes {
			return &Error{stage, fmt.Errorf("body exceeds js_body_limit")}
		}
	}
	name := "response"
	if stage == Request || stage == AfterRequestMap {
		name = "request"
	}
	if s.request != nil && name != "request" {
		if err := s.message("request", s.request, false, false); err != nil {
			return &Error{stage, err}
		}
	}
	if err := s.message(name, m, bodyMutable, stage != Response); err != nil {
		return &Error{stage, err}
	}
	output := cloneMessage(m)
	err := s.invoke(ctx, stage, func(v goja.Value) error {
		if !goja.IsUndefined(v) {
			return fmt.Errorf("hook must return undefined")
		}
		return s.applyMessage(name, output, bodyMutable)
	})
	if err == nil {
		m.Body = output.Body
		if m.Headers == nil {
			m.Headers = http.Header{}
		}
		clear(m.Headers)
		for key, values := range output.Headers {
			m.Headers[key] = values
		}
		if name == "request" {
			s.request = cloneMessage(m)
		} else {
			s.response = cloneMessage(m)
		}
	}
	return err
}
func cloneMessage(m *Message) *Message {
	if m == nil {
		return nil
	}
	copy := *m
	copy.Headers = m.Headers.Clone()
	return &copy
}

func (s *Session) invoke(ctx context.Context, stage Stage, validate func(goja.Value) error) (err error) {
	h := s.config.Handlers[stage]
	if h.program == nil {
		return &Error{stage, fmt.Errorf("handler was not prepared")}
	}
	timeout := s.config.Limits.Timeout
	if stage == SSEEvent {
		timeout = s.config.Limits.StreamTimeout
	}
	hookCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	s.stage = stage
	s.hookContext = hookCtx
	s.httpCalls = 0
	s.rejected = nil
	// Wait for an in-flight cancellation callback before clearing the interrupt,
	// otherwise it could interrupt the next stage on the same VM.
	interrupted := make(chan struct{})
	stop := context.AfterFunc(hookCtx, func() { s.vm.Interrupt(hookCtx.Err()); close(interrupted) })
	defer func() {
		if recovered := recover(); recovered != nil {
			if e, ok := recovered.(error); ok {
				err = e
			} else {
				panic(recovered)
			}
		}
		if !stop() {
			<-interrupted
		}
		s.vm.ClearInterrupt()
		if s.rejected != nil {
			err = s.rejected
		} else if err != nil {
			err = &Error{stage, sourceError(h.Location, h.Body, h.Line, h.Column, err)}
		}
	}()
	v, err := s.vm.RunProgram(h.program)
	if err != nil {
		return err
	}
	fn, ok := goja.AssertFunction(v)
	if !ok {
		return fmt.Errorf("handler is not a function")
	}
	ret, err := fn(goja.Undefined(), s.object)
	if err != nil {
		return err
	}
	if err = hookCtx.Err(); err != nil {
		return err
	}
	if validate != nil {
		err = validate(ret)
	}
	if err == nil {
		err = hookCtx.Err()
	}
	return err
}

// Finish runs at most once, with a fresh cleanup deadline even after cancellation.
// The host reports its error separately without replacing the attempt result.
func (s *Session) Finish(outcome Outcome) error {
	if s == nil || s.finished {
		return nil
	}
	s.finished = true
	delete(s.views, "event")
	if !s.Has(Log) {
		return nil
	}
	if s.request != nil {
		if err := s.message("request", s.request, false, false); err != nil {
			return err
		}
	}
	if s.response != nil {
		if err := s.message("response", s.response, false, false); err != nil {
			return err
		}
	}
	if err := s.readonly("result", s.jsonValue(outcome)); err != nil {
		return err
	}
	err := s.invoke(context.Background(), Log, func(v goja.Value) error {
		if !goja.IsUndefined(v) {
			return fmt.Errorf("log hook must return undefined")
		}
		return nil
	})
	return err
}
func sameRoutingBody(a, b string) bool {
	var x, y map[string]any
	if json.Unmarshal([]byte(a), &x) != nil || json.Unmarshal([]byte(b), &y) != nil {
		return a == b
	}
	for _, k := range []string{"model", "stream"} {
		if !reflect.DeepEqual(x[k], y[k]) {
			return false
		}
	}
	return true
}
func protectedHeader(k string) bool {
	switch strings.ToLower(k) {
	case "authorization", "proxy-authorization", "host", "content-length", "content-encoding", "transfer-encoding", "connection", "keep-alive", "te", "trailer", "upgrade", "x-api-key", "api-key", "x-goog-api-key", "cookie", "set-cookie":
		return true
	}
	return false
}
func visibleHeaders(h http.Header) map[string][]string {
	out := map[string][]string{}
	for k, v := range h {
		if !protectedHeader(k) {
			out[strings.ToLower(k)] = append([]string(nil), v...)
		}
	}
	return out
}
func validateHeaders(h map[string][]string) error {
	if len(h) > 256 {
		return fmt.Errorf("too many headers")
	}
	size := 0
	for k, vs := range h {
		if k == "" || k != strings.ToLower(k) || protectedHeader(k) {
			return fmt.Errorf("invalid or protected header: %s", k)
		}
		for _, c := range k {
			valid := c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", c)
			if !valid {
				return fmt.Errorf("invalid header name")
			}
		}
		size += len(k)
		for _, v := range vs {
			if strings.ContainsAny(v, "\r\n\x00") {
				return fmt.Errorf("invalid header value")
			}
			size += len(v)
		}
	}
	if size > 64<<10 {
		return fmt.Errorf("headers exceed 64 KiB")
	}
	return nil
}

// SealMessages freezes final message views before the first streamed event.
func (s *Session) SealMessages() error {
	if s == nil {
		return nil
	}
	if s.request != nil {
		if err := s.message("request", s.request, false, false); err != nil {
			return err
		}
	}
	if s.response != nil {
		return s.message("response", s.response, false, false)
	}
	return nil
}

func (s *Session) applyMessage(name string, m *Message, bodyMutable bool) error {
	obj := s.object.Get(name).ToObject(s.vm)
	body, ok := obj.Get("body").Export().(string)
	if !ok {
		return fmt.Errorf("body must be a string")
	}
	if bodyMutable {
		if !json.Valid([]byte(body)) || !utf8.ValidString(body) || int64(len(body)) > s.config.Limits.BodyBytes {
			return fmt.Errorf("invalid or oversized JSON body")
		}
		if name == "request" && !sameRoutingBody(m.Body, body) {
			return fmt.Errorf("model and stream cannot be changed by JavaScript")
		}
	}
	var headers map[string][]string
	if err := s.decodeHeaders(obj.Get("headers"), &headers); err != nil {
		return err
	}
	if !reflect.DeepEqual(headers["content-type"], visibleHeaders(m.Headers)["content-type"]) {
		return fmt.Errorf("content-type cannot be changed by JavaScript")
	}
	if err := validateHeaders(headers); err != nil {
		return err
	}
	m.Body = body
	for k := range m.Headers {
		if !protectedHeader(k) {
			delete(m.Headers, k)
		}
	}
	if m.Headers == nil {
		m.Headers = http.Header{}
	}
	for k, vs := range headers {
		m.Headers[http.CanonicalHeaderKey(k)] = vs
	}
	return nil
}

func (s *Session) decodeHeaders(v goja.Value, target *map[string][]string) error {
	var raw map[string][]json.RawMessage
	if err := s.decode(v, &raw); err != nil {
		return err
	}
	if raw == nil {
		return fmt.Errorf("headers must be an object")
	}
	headers := make(map[string][]string, len(raw))
	for name, values := range raw {
		if values == nil {
			return fmt.Errorf("header values must be string arrays")
		}
		decoded := make([]string, len(values))
		for i, value := range values {
			if len(value) == 0 || value[0] != '"' {
				return fmt.Errorf("header values must be strings")
			}
			if err := json.Unmarshal(value, &decoded[i]); err != nil {
				return err
			}
		}
		headers[name] = decoded
	}
	*target = headers
	return nil
}
