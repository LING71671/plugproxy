package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type failingSource struct {
	name string
}

func (s failingSource) Name() string {
	return s.name
}

func (s failingSource) Fetch(context.Context) ([]model.Proxy, error) {
	return nil, errors.New("boom")
}

func TestFetchWithOptionsWritesReportAndCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("one", []model.Proxy{
			{ID: "http://127.0.0.1:8080", Address: "127.0.0.1:8080", Protocol: model.ProtocolHTTP},
			{ID: "http://127.0.0.1:8080", Address: "127.0.0.1:8080", Protocol: model.ProtocolHTTP},
		}),
	})

	report := application.FetchWithOptions(context.Background(), FetchOptions{
		Workers:       1,
		CachePath:     path,
		CacheFallback: true,
		CacheWrite:    true,
	})

	if report.Added != 1 {
		t.Fatalf("expected 1 added proxy, got %d", report.Added)
	}
	if report.Duplicates != 1 {
		t.Fatalf("expected 1 duplicate, got %d", report.Duplicates)
	}
	if report.SuccessfulSources != 1 {
		t.Fatalf("expected 1 successful source, got %d", report.SuccessfulSources)
	}

	loaded, err := cache.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 cached proxy, got %d", len(loaded))
	}
}

func TestFetchWithOptionsFallsBackToCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	cached := []model.Proxy{{
		ID:       "http://127.0.0.1:8080",
		Address:  "127.0.0.1:8080",
		Protocol: model.ProtocolHTTP,
	}}
	if err := cache.Save(path, cached); err != nil {
		t.Fatal(err)
	}

	application := NewWithSources(slog.Default(), []source.Source{failingSource{name: "fail"}})
	report := application.FetchWithOptions(context.Background(), FetchOptions{
		Workers:       1,
		CachePath:     path,
		CacheFallback: true,
		CacheWrite:    true,
	})

	if !report.ReusedFromCache {
		t.Fatal("expected cache fallback")
	}
	if report.Added != 1 {
		t.Fatalf("expected 1 proxy from cache, got %d", report.Added)
	}
	if report.FailedSources != 1 {
		t.Fatalf("expected 1 failed source, got %d", report.FailedSources)
	}
	if len(application.Pool().List(pool.Filter{})) != 1 {
		t.Fatal("expected cached proxy in pool")
	}
}
