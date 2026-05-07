package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LING71671/plugproxy/internal/source"
)

func TestBuildSourcesSkipsDisabled(t *testing.T) {
	disabled := false
	enabled := true
	sources, err := BuildSources(Config{Sources: []SourceConfig{
		{Name: "off", Type: "raw_text_url", URL: "https://example.com/off.txt", Enabled: &disabled},
		{Name: "on", Type: "raw_text_url", URL: "https://example.com/on.txt", Enabled: &enabled},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if sources[0].Name() != "on" {
		t.Fatalf("expected enabled source, got %s", sources[0].Name())
	}
}

func TestBuildSourcesSupportsJSONAndAPI(t *testing.T) {
	enabled := true
	sources, err := BuildSources(Config{Sources: []SourceConfig{
		{Name: "json", Type: "json_url", URL: "https://example.com/proxies.json", Enabled: &enabled},
		{Name: "api", Type: "api_url", URL: "https://api.example.com/free-proxies", Headers: map[string]string{"Accept": "application/json"}, JSON: &source.JSONConfig{ItemsPath: "data"}, Enabled: &enabled},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Name() != "json" || sources[1].Name() != "api" {
		t.Fatalf("unexpected sources %#v", sources)
	}
}

func TestBuildSourcesSupportsHTMLText(t *testing.T) {
	enabled := true
	sources, err := BuildSources(Config{Sources: []SourceConfig{
		{Name: "html", Type: "html_text_url", URL: "https://example.com/free", Enabled: &enabled},
		{Name: "br", Type: "br_text_url", URL: "https://example.com/api", Enabled: &enabled},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Name() != "html" || sources[1].Name() != "br" {
		t.Fatalf("unexpected sources %#v", sources)
	}
}

func TestLoadParsesJSONConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sources.json")
	data := []byte(`{"sources":[{"name":"test","type":"raw_text_url","url":"https://example.com/proxies.txt","protocol_hint":"http","enabled":true}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) != 1 || cfg.Sources[0].Name != "test" {
		t.Fatalf("unexpected config %#v", cfg)
	}
}

func TestLoadMissingDefaultReturnsDefaultConfig(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), DefaultPath))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Sources) == 0 {
		t.Fatal("expected default sources")
	}
}

func TestSaveWritesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sources.json")
	cfg := Config{Sources: []SourceConfig{{Name: "test", Type: "raw_text_url", URL: "https://example.com/proxies.txt"}}}

	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Sources) != 1 || loaded.Sources[0].Name != "test" {
		t.Fatalf("unexpected loaded config %#v", loaded)
	}
}
