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
