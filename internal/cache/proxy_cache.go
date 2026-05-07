package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

const DefaultPath = ".plugproxy.cache.json"

type ProxyCache struct {
	Version int           `json:"version"`
	SavedAt time.Time     `json:"saved_at"`
	Proxies []model.Proxy `json:"proxies"`
}

type Stats struct {
	Path    string           `json:"path"`
	Version int              `json:"version"`
	SavedAt time.Time        `json:"saved_at,omitempty"`
	Count   int              `json:"count"`
	Pool    model.ProxyStats `json:"pool"`
}

type CompactOptions struct {
	MaxEntries     int
	DropDeadAfter  time.Duration
	DropStaleAfter time.Duration
	Now            time.Time
}

type CompactReport struct {
	Path    string `json:"path"`
	Before  int    `json:"before"`
	After   int    `json:"after"`
	Removed int    `json:"removed"`
}

type RepairReport struct {
	Path    string `json:"path"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	BadPath string `json:"bad_path,omitempty"`
}

func Load(path string) ([]model.Proxy, error) {
	cache, err := LoadCache(path)
	if err != nil {
		return nil, err
	}
	return cache.Proxies, nil
}

func LoadCache(path string) (ProxyCache, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ProxyCache{}, err
	}

	cache, err := decodeCache(data)
	if err != nil {
		badPath, moveErr := quarantineBad(path)
		if moveErr != nil {
			return ProxyCache{}, fmt.Errorf("decode cache: %w; quarantine failed: %v", err, moveErr)
		}
		return ProxyCache{}, fmt.Errorf("decode cache: %w; moved bad cache to %s", err, badPath)
	}
	return cache, nil
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

func FileStats(path string) (Stats, error) {
	cache, err := LoadCache(path)
	if err != nil {
		return Stats{}, err
	}
	if path == "" {
		path = DefaultPath
	}
	return Stats{
		Path:    path,
		Version: cache.Version,
		SavedAt: cache.SavedAt,
		Count:   len(cache.Proxies),
		Pool:    model.NewProxyStats(cache.Proxies),
	}, nil
}

func Compact(path string, options CompactOptions) (CompactReport, error) {
	if path == "" {
		path = DefaultPath
	}
	if options.Now.IsZero() {
		options.Now = time.Now()
	}
	proxies, err := Load(path)
	if err != nil {
		return CompactReport{}, err
	}
	before := len(proxies)
	kept := make([]model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if options.DropDeadAfter > 0 && proxy.Status() == model.HealthDead && !proxy.LastFailureAt.IsZero() && options.Now.Sub(proxy.LastFailureAt) > options.DropDeadAfter {
			continue
		}
		if options.DropStaleAfter > 0 && !proxy.LastSeenAt.IsZero() && options.Now.Sub(proxy.LastSeenAt) > options.DropStaleAfter {
			continue
		}
		kept = append(kept, proxy)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return cacheKeepRank(kept[i]) > cacheKeepRank(kept[j])
	})
	if options.MaxEntries > 0 && len(kept) > options.MaxEntries {
		kept = kept[:options.MaxEntries]
	}
	if err := Save(path, kept); err != nil {
		return CompactReport{}, err
	}
	return CompactReport{Path: path, Before: before, After: len(kept), Removed: before - len(kept)}, nil
}

func Repair(path string) RepairReport {
	if path == "" {
		path = DefaultPath
	}
	_, err := LoadCache(path)
	if err == nil {
		return RepairReport{Path: path, OK: true}
	}
	report := RepairReport{Path: path, OK: false, Error: err.Error()}
	if matches, _ := filepath.Glob(path + ".bad*"); len(matches) > 0 {
		report.BadPath = matches[len(matches)-1]
	}
	return report
}

func decodeCache(data []byte) (ProxyCache, error) {
	var cache ProxyCache
	if err := json.Unmarshal(data, &cache); err == nil && cache.Proxies != nil {
		if cache.Version == 0 {
			cache.Version = 1
		}
		return cache, nil
	}
	var proxies []model.Proxy
	if err := json.Unmarshal(data, &proxies); err != nil {
		return ProxyCache{}, err
	}
	return ProxyCache{Version: 1, Proxies: proxies}, nil
}

func quarantineBad(path string) (string, error) {
	badPath := path + ".bad"
	if _, err := os.Stat(badPath); err == nil {
		badPath = fmt.Sprintf("%s.bad.%d", path, time.Now().UnixNano())
	}
	if err := os.Rename(path, badPath); err != nil {
		return "", err
	}
	return badPath, nil
}

func cacheKeepRank(proxy model.Proxy) int {
	rank := 0
	switch proxy.Status() {
	case model.HealthHealthy:
		rank += 4000
	case model.HealthDegraded:
		rank += 3000
	case model.HealthUnchecked:
		rank += 2000
	default:
		rank += 1000
	}
	rank += proxy.HealthScore * 10
	rank += proxy.SeenCount
	if !proxy.LastSeenAt.IsZero() {
		rank += 5
	}
	return rank
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
