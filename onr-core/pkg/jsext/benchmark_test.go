package jsext

import (
	"context"
	"io"
	"sort"
	"testing"
	"time"
)

func BenchmarkHooks(b *testing.B) {
	cases := []struct {
		name   string
		stage  Stage
		body   string
		events int
	}{
		{name: "disabled"},
		{name: "log", stage: Log, body: `ctx.state.done=true;`},
		{name: "json", stage: Request, body: `const body=JSON.parse(ctx.request.body);body.checked=true;ctx.request.body=JSON.stringify(body);`},
		{name: "sse_256", stage: SSEEvent, body: `ctx.state.events=(ctx.state.events||0)+1;return ctx.event;`, events: 256},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			p := Provider{Defaults: Config{Limits: Limits{Timeout: time.Second, StreamTimeout: time.Second}}}
			if tc.stage != "" {
				p.Defaults.Handlers = map[Stage]HandlerSource{tc.stage: {Body: tc.body}}
			}
			prepared, err := p.Prepare(RuntimeConfig{}, "bench")
			if err != nil {
				b.Fatal(err)
			}
			cfg := prepared.Select("chat.completions", false)
			samples := make([]int64, 0, 10000)
			var firstEventTotal time.Duration
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				start := time.Now()
				s, err := NewSession(cfg, Meta{}, HTTPPolicy{}, nil)
				if err != nil {
					b.Fatal(err)
				}
				switch tc.stage {
				case Request:
					if err := s.Run(context.Background(), Request, message()); err != nil {
						b.Fatal(err)
					}
				case SSEEvent:
					w := NewSSEWriter(context.Background(), s, io.Discard, TerminalForAPI("chat.completions"))
					for i := 0; i < tc.events; i++ {
						if _, err := w.Write([]byte("data: {\"delta\":\"token\"}\n\n")); err != nil {
							b.Fatal(err)
						}
						if i == 0 {
							firstEventTotal += time.Since(start)
						}
					}
					if err := w.Finish(); err != nil {
						b.Fatal(err)
					}
				}
				if err := s.Finish(Outcome{Outcome: "success"}); err != nil {
					b.Fatal(err)
				}
				if len(samples) < cap(samples) {
					samples = append(samples, time.Since(start).Nanoseconds())
				}
			}
			b.StopTimer()
			sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
			if len(samples) > 0 {
				for _, p := range []struct {
					name    string
					percent int
				}{{"p50-ns", 50}, {"p95-ns", 95}, {"p99-ns", 99}} {
					b.ReportMetric(float64(samples[(len(samples)-1)*p.percent/100]), p.name)
				}
			}
			if tc.events > 0 {
				b.ReportMetric(float64(firstEventTotal.Nanoseconds())/float64(b.N), "first-event-ns")
			}
		})
	}
}
