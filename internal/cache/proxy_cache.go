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
	return writeFileAtomic(path, data, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	file, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(perm); err != nil {
		_ = file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	_ = file.Sync()
	if err := file.Close(); err != nil {
		return err
	}

	if err := replaceFile(tempPath, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
