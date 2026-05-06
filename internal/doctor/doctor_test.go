package doctor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestRunReportsConfigAndCache(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sources.json")
	cachePath := filepath.Join(dir, "cache.json")

	cfg := config.Config{Sources: []config.SourceConfig{{Name: "test", Type: "raw_text_url", URL: "https://example.com/proxies.txt"}}}
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	if err := cache.Save(cachePath, []model.Proxy{{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy}}); err != nil {
		t.Fatal(err)
	}

	report := Run(context.Background(), Options{ConfigPath: configPath, CachePath: cachePath})
	if !report.OK {
		t.Fatalf("expected report ok, got %#v", report)
	}
	if report.Config.Enabled != 1 {
		t.Fatalf("expected 1 enabled source, got %d", report.Config.Enabled)
	}
	if report.Cache.Proxies != 1 || report.Cache.Stats.Healthy != 1 {
		t.Fatalf("unexpected cache report %#v", report.Cache)
	}
}

func TestRunReportsInvalidConfigAsFailure(t *testing.T) {
	report := Run(context.Background(), Options{ConfigPath: filepath.Join(t.TempDir(), "missing.json")})
	if report.OK {
		t.Fatal("expected report failure")
	}
	if report.Checks[0].Status != StatusFail {
		t.Fatalf("expected config failure, got %#v", report.Checks)
	}
}

func TestRunChecksAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("expected /health, got %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	report := Run(context.Background(), Options{APIURL: server.URL})
	if report.API.Status != StatusOK {
		t.Fatalf("expected api ok, got %#v", report.API)
	}
}
