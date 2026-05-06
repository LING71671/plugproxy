package cache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestSaveAndLoadProxyCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	proxies := []model.Proxy{{
		ID:       "http://127.0.0.1:8080",
		Address:  "127.0.0.1:8080",
		Protocol: model.ProtocolHTTP,
	}}

	if err := Save(path, proxies); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(loaded))
	}
	if loaded[0].ID != proxies[0].ID {
		t.Fatalf("expected proxy %s, got %s", proxies[0].ID, loaded[0].ID)
	}
}

func TestSaveProxyCacheDoesNotLeaveTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.json")
	if err := Save(path, []model.Proxy{{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP}}); err != nil {
		t.Fatal(err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".cache.json.*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temp files, got %#v", matches)
	}
}

func TestSaveProxyCacheReplaceFailureKeepsExistingFile(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "cache.json")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Save(targetDir, []model.Proxy{{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP}})
	if err == nil {
		t.Fatal("expected replace failure")
	}
	info, statErr := os.Stat(targetDir)
	if statErr != nil {
		t.Fatal(statErr)
	}
	if !info.IsDir() {
		t.Fatal("expected existing directory to remain")
	}
}

func TestLoadOldProxyCacheWithoutSeenFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	data := []byte(`{"version":1,"proxies":[{"id":"http://127.0.0.1:8080","address":"127.0.0.1:8080","protocol":"http"}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].SeenCount != 0 || !loaded[0].LastSeenAt.IsZero() {
		t.Fatalf("expected old cache to load without seen fields, got %#v", loaded)
	}
}
