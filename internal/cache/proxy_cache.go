package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

const DefaultPath = ".plugproxy.cache.json"

type ProxyCache struct {
	Version int           `json:"version"`
	SavedAt time.Time     `json:"saved_at"`
	Proxies []model.Proxy `json:"proxies"`
}

func Load(path string) ([]model.Proxy, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cache ProxyCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return cache.Proxies, nil
}

func Save(path string, proxies []model.Proxy) error {
	if path == "" {
		path = DefaultPath
	}

	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create cache dir: %w", err)
		}
	}

	data, err := json.MarshalIndent(ProxyCache{
		Version: 1,
		SavedAt: time.Now(),
		Proxies: proxies,
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
