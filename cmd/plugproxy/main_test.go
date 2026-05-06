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

	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/discover"
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
	})

	if len(cfg.Sources) != 1 {
		t.Fatalf("expected one exported source, got %#v", cfg)
	}
	item := cfg.Sources[0]
	if item.Type != "json_url" || item.Enabled == nil || *item.Enabled || item.JSON == nil {
		t.Fatalf("unexpected exported source %#v", item)
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
