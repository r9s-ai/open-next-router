package jsext

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testSession(t *testing.T, handlers map[Stage]string, limits Limits, logger Logger) *Session {
	t.Helper()
	p := Provider{Defaults: Config{Handlers: map[Stage]HandlerSource{}, Limits: limits}}
	for stage, body := range handlers {
		p.Defaults.Handlers[stage] = HandlerSource{Body: body, Location: "test.js"}
	}
	p, err := p.Prepare(RuntimeConfig{}, "test")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewSession(p.Select("chat.completions", false), Meta{Provider: "test", Attempt: 1}, HTTPPolicy{}, logger)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func message() *Message {
	return &Message{Body: `{"model":"m","stream":false,"value":1}`, ContentType: "application/json", Headers: http.Header{"Authorization": {"secret"}, "X-Test": {"a", "b"}}}
}
func TestSessionLifecycleAndIsolation(t *testing.T) {
	var logs int
	s := testSession(t, map[Stage]string{Request: `if(ctx.request.headers.authorization) throw new Error("secret"); ctx.state.n=3; let b=JSON.parse(ctx.request.body); b.value=2; ctx.request.body=JSON.stringify(b);`, AfterRequestMap: `ctx.request.headers["x-count"]=[String(ctx.state.n)];`, Log: `ctx.log.info(ctx.result.outcome,{n:ctx.state.n});`}, Limits{}, func(level, msg string, fields map[string]any) {
		logs++
		if msg != "success" || fields["n"].(float64) != 3 {
			t.Errorf("wrong log %s %v", msg, fields)
		}
	})
	m := message()
	if err := s.Run(context.Background(), Request, m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Body, `"value":2`) || m.Headers.Get("Authorization") != "secret" {
		t.Fatal(m)
	}
	if err := s.Run(context.Background(), AfterRequestMap, m); err != nil {
		t.Fatal(err)
	}
	if m.Headers.Get("X-Count") != "3" {
		t.Fatal(m.Headers)
	}
	if err := s.Finish(Outcome{Outcome: "success"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Finish(Outcome{}); err != nil {
		t.Fatal(err)
	}
	if logs != 1 {
		t.Fatal(logs)
	}
}
func TestSessionRejectsUnsafeChanges(t *testing.T) {
	for name, body := range map[string]string{
		"route": `ctx.meta.provider="evil";`, "redefine": `Object.defineProperty(ctx,"meta",{value:{}});`, "model": `ctx.request.body='{"model":"evil"}';`, "auth": `ctx.request.headers.authorization=["evil"];`, "header_value": `ctx.request.headers["x-a"]=["a\r\nb"];`, "wrong_headers": `ctx.request.headers["x-a"]="a";`, "return": `return 42;`, "infinite": `while(true){}`, "promise": `return Promise.resolve();`, "wrong_json": `ctx.request.body="broken";`, "limit": `ctx.request.body=JSON.stringify({x:"x".repeat(2000)});`, "function": `new Function("return 1")();`, "getter": `Object.defineProperty(ctx.request.headers,"x-z",{enumerable:true,get(){while(true){}}});`,
	} {
		t.Run(name, func(t *testing.T) {
			s := testSession(t, map[Stage]string{Request: body}, Limits{Timeout: 20 * time.Millisecond, BodyBytes: 512}, nil)
			m := message()
			before := m.Body
			if err := s.Run(context.Background(), Request, m); err == nil {
				t.Fatal("accepted unsafe script")
			}
			if m.Body != before {
				t.Fatal("failed hook mutated host body")
			}
		})
	}
}
func TestRejectionAndCancellationStillLog(t *testing.T) {
	for _, body := range []string{`ctx.reject(403,{error:"blocked"});`, `while(true){}`} {
		count := 0
		s := testSession(t, map[Stage]string{Request: body, Log: `ctx.log.info("done");`}, Limits{Timeout: 10 * time.Millisecond}, func(string, string, map[string]any) { count++ })
		err := s.Run(context.Background(), Request, message())
		if !Terminal(err) {
			t.Fatalf("not terminal: %v", err)
		}
		if strings.Contains(body, "reject") {
			var r *Rejection
			if !errors.As(err, &r) || r.Status != 403 {
				t.Fatal(err)
			}
		}
		if err := s.Finish(Outcome{Outcome: "script_error"}); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatal(count)
		}
	}
}
func TestConcurrentSessions(t *testing.T) {
	p := Provider{Defaults: Config{Handlers: map[Stage]HandlerSource{Request: {Body: `ctx.state.count=(ctx.state.count||0)+1; if(ctx.state.count!==1)throw new Error("leak");`}}}}
	p, err := p.Prepare(RuntimeConfig{}, "p")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() {
			s, err := NewSession(p.Select("a", false), Meta{}, HTTPPolicy{}, nil)
			if err == nil {
				err = s.Run(context.Background(), Request, message())
			}
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
}
func TestFilesAndAsync(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.js")
	if err := os.WriteFile(outside, []byte("return;"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape.js")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ok.js"), []byte("return;"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../outside.js", outside, "escape.js", "missing.js"} {
		if _, err := readScript(root, name); err == nil {
			t.Fatal(name)
		}
	}
	if _, err := readScript(root, "ok.js"); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`async function f(){return 1}`, `const f=async ()=>1;`, `import x from "x";`} {
		if _, err := Compile("test", body); err == nil {
			t.Fatal(body)
		}
	}
}
func TestSSEFramesAndTerminal(t *testing.T) {
	s := testSession(t, map[Stage]string{SSEEvent: `const e=ctx.event;if(e.data==="drop")return null; e.data=e.data.toUpperCase();return [e,e];`}, Limits{}, nil)
	var out bytes.Buffer
	w := NewSSEWriter(context.Background(), s, &out, TerminalForAPI("chat.completions"))
	source := ": heartbeat\r\n\r\ndata: a\r\ndata: b\r\n\r\ndata: drop\n\ndata: [DONE]\n\n"
	for _, b := range []byte(source) {
		if _, err := w.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	if out.String() != ": heartbeat\r\n\r\ndata: A\ndata: B\n\ndata: A\ndata: B\n\ndata: [DONE]\n\n" {
		t.Fatalf("%q", out.String())
	}
	for _, body := range []string{`return;`, `return {data:"[DONE]"};`, `return {event:"bad\nx",data:"x"};`, `return Array(65).fill(ctx.event);`, `return {data:1};`} {
		s := testSession(t, map[Stage]string{SSEEvent: body}, Limits{}, nil)
		w := NewSSEWriter(context.Background(), s, &out, TerminalForAPI("responses"))
		if _, err := w.Write([]byte("data: x\n\n")); err == nil {
			t.Fatal(body)
		}
	}
}
func TestHTTPGuards(t *testing.T) {
	for _, raw := range []string{"127.0.0.1", "::1", "10.1.2.3", "169.254.169.254", "100.64.0.1", "::ffff:127.0.0.1", "2001:db8::1"} {
		if publicIP(netip.MustParseAddr(raw)) {
			t.Fatal(raw)
		}
	}
	if !publicIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("public address rejected")
	}
	s := testSession(t, map[Stage]string{Request: `ctx.http.request({url:"http://127.0.0.1/x"});`}, Limits{}, nil)
	if err := s.Run(context.Background(), Request, message()); err == nil {
		t.Fatal("unauthorized request accepted")
	}
	s.policy = HTTPPolicy{Enabled: true, AllowedOrigins: []string{"http://127.0.0.1"}}.clone()
	if err := s.Run(context.Background(), Request, message()); err == nil {
		t.Fatal("private destination accepted")
	}
}
func BenchmarkSession(b *testing.B) {
	for _, script := range []bool{false, true} {
		name := "disabled"
		if script {
			name = "log"
		}
		b.Run(name, func(b *testing.B) {
			p := Provider{}
			if script {
				p.Defaults.Handlers = map[Stage]HandlerSource{Log: {Body: `return;`}}
			}
			p, _ = p.Prepare(RuntimeConfig{}, "p")
			cfg := p.Select("a", false)
			b.ReportAllocs()
			for b.Loop() {
				s, err := NewSession(cfg, Meta{}, HTTPPolicy{}, nil)
				if err != nil {
					b.Fatal(err)
				}
				if err := s.Finish(Outcome{Outcome: "success"}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
