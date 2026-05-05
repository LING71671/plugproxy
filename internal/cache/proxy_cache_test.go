package cache

import (
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
