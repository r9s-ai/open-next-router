package usageestimate

import "testing"

func TestValidate_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "default_ok", cfg: Config{}, wantErr: false},
		{name: "strategy_ok_empty", cfg: Config{Strategy: ""}, wantErr: false},
		{name: "strategy_ok_heuristic", cfg: Config{Strategy: "heuristic"}, wantErr: false},
		{name: "strategy_invalid", cfg: Config{Strategy: "foo"}, wantErr: true},
		{name: "max_request_negative", cfg: Config{MaxRequestBytes: -1}, wantErr: true},
		{name: "max_response_negative", cfg: Config{MaxResponseBytes: -1}, wantErr: true},
		{name: "max_stream_negative", cfg: Config{MaxStreamCollectBytes: -1}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(&tc.cfg)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsAPIEnabled_TableDriven(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	ApplyDefaults(cfg)

	cases := []struct {
		name string
		api  string
		want bool
	}{
		{name: "enabled_default_chat", api: "chat.completions", want: true},
		{name: "enabled_case_insensitive", api: "Chat.Completions", want: true},
		{name: "disabled_unknown", api: "unknown.api", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cfg.IsAPIEnabled(tc.api); got != tc.want {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}

	cfg.Enabled = false
	if got := cfg.IsAPIEnabled("chat.completions"); got {
		t.Fatalf("expected disabled config to disable api")
	}
}
