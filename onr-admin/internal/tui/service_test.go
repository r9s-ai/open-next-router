package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/r9s-ai/open-next-router/pkg/config"
)

func TestAdminServiceSnapshotWithoutRedis(t *testing.T) {
	service := &adminService{cfg: &config.Config{}}
	snapshot := service.Snapshot(context.Background())
	if snapshot.RedisEnabled || snapshot.RedisReachable || snapshot.MeterryEnabled {
		t.Fatalf("unexpected disabled snapshot: %+v", snapshot)
	}
}

func TestMeterryReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("method=%s, want HEAD", r.Method)
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	if reachable, errText := meterryReachable(context.Background(), server.URL); !reachable || errText != "" {
		t.Fatalf("reachable=(%v,%q)", reachable, errText)
	}
}
