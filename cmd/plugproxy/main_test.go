package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/discover"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestReorderFlagArgsSupportsNewRunFlags(t *testing.T) {
	args := reorderFlagArgs([]string{
		"run",
		"-refresh=true",
		"-source-cooldown", "1m",
		"-per-host-workers", "2",
		"-connect-timeout", "3s",
		"-shutdown-timeout", "5s",
		"-log-level", "debug",
		"-check-profile", "smart",
		"-protocol-fair=true",
		"-skip-check=false",
	}, map[string]bool{
		"refresh": true, "source-cooldown": false, "per-host-workers": false,
		"connect-timeout": false, "shutdown-timeout": false, "log-level": false,
		"check-profile": false, "protocol-fair": true, "skip-check": true,
	})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-source-cooldown 1m") ||
		!strings.Contains(joined, "-per-host-workers 2") ||
		!strings.Contains(joined, "-connect-timeout 3s") ||
		!strings.Contains(joined, "-shutdown-timeout 5s") ||
		!strings.Contains(joined, "-log-level debug") ||
		!strings.Contains(joined, "-check-profile smart") ||
		!strings.Contains(joined, "-protocol-fair=true") {
		t.Fatalf("new flags were not preserved: %#v", args)
	}
}

func TestSchedulerProfileParser(t *testing.T) {
	if schedulerProfile("smart") != "smart" {
		t.Fatal("expected smart profile")
	}
	if schedulerProfile("bad") != "full" {
		t.Fatal("expected invalid profile to fall back to full")
	}
}

func TestFlagWasSet(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	value := fs.Bool("skip-unsupported", false, "")
	if err := fs.Parse([]string{"-skip-unsupported=false"}); err != nil {
		t.Fatal(err)
	}
	if !flagWasSet(fs, "skip-unsupported") || *value {
		t.Fatalf("expected explicit false flag to be detected")
	}
}

func TestSlogLevelParser(t *testing.T) {
	if slogLevel("debug") != slog.LevelDebug {
		t.Fatal("expected debug level")
	}
	if slogLevel("bad") != slog.LevelInfo {
		t.Fatal("expected invalid level to fall back to info")
	}
}

func TestSourcesConfigFromCandidatesExportsDisabledSupportedSources(t *testing.T) {
	cfg := sourcesConfigFromCandidates([]discover.CandidateSource{
		{
			URL:             "https://example.com/proxies.json",
			SourceKind:      discover.KindJSON,
			Status:          discover.StatusValid,
			AdapterRequired: false,
			ProtocolHint:    "http",
			Recipe:          &discover.SourceRecipe{Kind: discover.KindJSON, Parser: "json_auto"},
		},
		{
			URL:             "https://example.com/table.html",
			SourceKind:      discover.KindHTMLTable,
			Status:          discover.StatusValid,
			AdapterRequired: true,
		},
		{
			URL:             "https://example.com/br.html",
			SourceKind:      discover.KindHTMLText,
			Status:          discover.StatusValid,
			AdapterRequired: false,
		},
	})

	if len(cfg.Sources) != 2 {
		t.Fatalf("expected two exported sources, got %#v", cfg)
	}
	item := cfg.Sources[0]
	if item.Type != "json_url" || item.Enabled == nil || *item.Enabled || item.JSON == nil {
		t.Fatalf("unexpected exported source %#v", item)
	}
	if cfg.Sources[1].Type != "html_text_url" || cfg.Sources[1].Enabled == nil || *cfg.Sources[1].Enabled {
		t.Fatalf("unexpected html exported source %#v", cfg.Sources[1])
	}
}

func TestRunDiscoverValidateWritesSources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`["127.0.0.1:8080"]`))
	}))
	defer server.Close()

	inputPath := filepath.Join(t.TempDir(), "candidates.json")
	outputPath := filepath.Join(t.TempDir(), "sources.json")
	report := discover.DiscoveryReport{Candidates: []discover.CandidateSource{{
		URL:             server.URL,
		SourceKind:      discover.KindJSON,
		Format:          discover.FormatJSON,
		Status:          discover.StatusCandidate,
		AdapterRequired: false,
	}}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runDiscover(context.Background(), []string{"validate", "-workers", "1", "-per-host-workers", "1", "-write-sources", outputPath, inputPath}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].Type != "json_url" || cfg.Sources[0].Enabled == nil || *cfg.Sources[0].Enabled {
		t.Fatalf("unexpected exported config %#v", cfg)
	}
}

func TestRunConfigCommandInitValidatePrint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugproxy.config.json")
	if err := runConfigCommand([]string{"init", "-app-config", path}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigCommand([]string{"validate", "-app-config", path}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigCommand([]string{"print", "-app-config", path}); err != nil {
		t.Fatal(err)
	}
}

func TestRunCacheCommandStatsCompactRepair(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := cache.Save(path, []model.Proxy{
		{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy},
		{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded},
	}); err != nil {
		t.Fatal(err)
	}
	if err := runCacheCommand([]string{"stats", "-cache", path}); err != nil {
		t.Fatal(err)
	}
	if err := runCacheCommand([]string{"compact", "-cache", path, "-max-entries", "1"}); err != nil {
		t.Fatal(err)
	}
	if err := runCacheCommand([]string{"repair", "-cache", path}); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected compacted cache, got %#v", loaded)
	}
}

func TestRunWatchPollsMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metrics.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"pool":    map[string]any{"total": 1, "healthy": 1, "degraded": 0, "dead": 0, "unchecked": 0},
			"check":   map[string]any{"scheduled": 1, "failed": 0},
			"refresh": map[string]any{"status": "idle", "phase": "idle", "last_reason": "test"},
		})
	}))
	defer server.Close()

	if err := runWatch(context.Background(), []string{"-api", server.URL, "-count", "1", "-interval", "1ms"}); err != nil {
		t.Fatal(err)
	}
}

func TestLoadAppDefaultsUsesExplicitConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plugproxy.config.json")
	cfg := config.DefaultAppConfig()
	cfg.Fetch.SourceWorkers = 7
	if err := config.SaveApp(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, loadedPath, ok, err := loadAppDefaults([]string{"-app-config", path})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || loadedPath != path || loaded.Fetch.SourceWorkers != 7 {
		t.Fatalf("unexpected app defaults %#v %s %t", loaded, loadedPath, ok)
	}
}

func TestRunSourcesCommandEnableDisableAddRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.json")
	if err := config.Save(path, config.Config{}); err != nil {
		t.Fatal(err)
	}
	if err := runSourcesCommand(context.Background(), []string{"add", "-config", path, "-name", "one", "-url", "https://example.com/proxies.txt", "-protocol-hint", "http"}); err != nil {
		t.Fatal(err)
	}
	if err := runSourcesCommand(context.Background(), []string{"enable", "-config", path, "one"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].Enabled == nil || !*cfg.Sources[0].Enabled {
		t.Fatalf("expected enabled source, got %#v", cfg)
	}
	if err := runSourcesCommand(context.Background(), []string{"disable", "-config", path, "one"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Sources[0].Enabled == nil || *cfg.Sources[0].Enabled {
		t.Fatalf("expected disabled source, got %#v", cfg.Sources[0])
	}
	if err := runSourcesCommand(context.Background(), []string{"remove", "-config", path, "one"}); err != nil {
		t.Fatal(err)
	}
	cfg, err = config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 0 {
		t.Fatalf("expected source removed, got %#v", cfg)
	}
}
