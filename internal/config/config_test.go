package config

import (
	"os"
	"path/filepath"
	"testing"
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
