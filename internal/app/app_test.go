package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/scheduler"
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

type blockingSource struct {
	name    string
	started chan struct{}
	release chan struct{}
}

func (s blockingSource) Name() string {
	return s.name
}

func (s blockingSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	close(s.started)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		return []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}, nil
	}
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

func TestFetchWithOptionsPreservesCachedHealth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	checkedAt := time.Now().Add(-time.Minute).UTC()
	if err := cache.Save(path, []model.Proxy{{
		ID:            "http://127.0.0.1:8080",
		Address:       "127.0.0.1:8080",
		Protocol:      model.ProtocolHTTP,
		HealthStatus:  model.HealthHealthy,
		HealthScore:   90,
		CheckCount:    3,
		LastCheckedAt: checkedAt,
	}}); err != nil {
		t.Fatal(err)
	}

	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("fresh", []model.Proxy{{ID: "http://127.0.0.1:8080", Address: "127.0.0.1:8080", Protocol: model.ProtocolHTTP}}),
	})
	application.FetchWithOptions(context.Background(), FetchOptions{
		Workers:       1,
		CachePath:     path,
		CacheFallback: true,
		CacheWrite:    true,
	})

	items := application.Pool().List(pool.Filter{})
	if len(items) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(items))
	}
	if items[0].HealthStatus != model.HealthHealthy || items[0].HealthScore != 90 || items[0].CheckCount != 3 {
		t.Fatalf("expected cached health to be preserved, got %#v", items[0])
	}
	if items[0].SeenCount != 1 || items[0].LastSeenAt.IsZero() {
		t.Fatalf("expected seen metadata to be updated, got %#v", items[0])
	}
}

func TestCheckWithOptionsWritesCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	application := NewWithSources(slog.Default(), nil)
	application.Pool().Add(model.Proxy{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4})

	stats := application.CheckWithOptions(context.Background(), CheckOptions{
		Workers:    1,
		Filter:     pool.Filter{Protocol: model.ProtocolSOCKS4},
		CachePath:  path,
		CacheWrite: true,
	})
	if stats.Unsupported != 1 {
		t.Fatalf("expected 1 unsupported proxy, got %d", stats.Unsupported)
	}
	if stats.ErrorTypes["protocol_unsupported"] != 1 {
		t.Fatalf("expected protocol_unsupported error type, got %#v", stats.ErrorTypes)
	}

	loaded, err := cache.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 cached proxy, got %d", len(loaded))
	}
	if loaded[0].CheckCount != 1 || loaded[0].LastError == "" {
		t.Fatalf("expected checked health fields, got %#v", loaded[0])
	}

	metrics := application.Metrics(RefreshOptions{})
	if metrics.Check.Unsupported != 1 || metrics.Runtime.Goroutines == 0 || metrics.UptimeMS < 0 {
		t.Fatalf("expected check metrics to be updated, got %#v", metrics)
	}
}

func TestCheckWithOptionsSkipsRecentByTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	checkedAt := time.Now().Add(-time.Minute).UTC()
	application := NewWithSources(slog.Default(), nil)
	application.Pool().Add(model.Proxy{
		ID:            "socks4://127.0.0.1:1080",
		Address:       "127.0.0.1:1080",
		Protocol:      model.ProtocolSOCKS4,
		HealthStatus:  model.HealthDead,
		HealthScore:   10,
		CheckCount:    1,
		LastCheckedAt: checkedAt,
	})

	stats := application.CheckWithOptions(context.Background(), CheckOptions{
		Workers:    1,
		Filter:     pool.Filter{Protocol: model.ProtocolSOCKS4},
		CachePath:  path,
		CacheWrite: true,
		CheckTTL:   30 * time.Minute,
	})
	if stats.Total != 1 || stats.Scheduled != 0 || stats.SkippedRecent != 1 {
		t.Fatalf("unexpected schedule stats %#v", stats)
	}
	if stats.Unsupported != 0 {
		t.Fatalf("expected skipped proxy not to be checked, got %#v", stats)
	}

	loaded, err := cache.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].CheckCount != 1 || !loaded[0].LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected cached proxy unchanged, got %#v", loaded)
	}
}

func TestCheckWithOptionsHonorsMaxChecks(t *testing.T) {
	application := NewWithSources(slog.Default(), nil)
	application.Pool().Add(model.Proxy{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4})
	application.Pool().Add(model.Proxy{ID: "socks4://127.0.0.2:1080", Address: "127.0.0.2:1080", Protocol: model.ProtocolSOCKS4})

	stats := application.CheckWithOptions(context.Background(), CheckOptions{
		Workers:   1,
		Filter:    pool.Filter{Protocol: model.ProtocolSOCKS4},
		MaxChecks: 1,
	})
	if stats.Total != 2 || stats.Scheduled != 1 || stats.SkippedLimit != 1 {
		t.Fatalf("unexpected schedule stats %#v", stats)
	}
	if stats.Unsupported != 1 {
		t.Fatalf("expected one scheduled socks4 check, got %#v", stats)
	}
}

func TestCheckWithOptionsSmartSkipsUnsupported(t *testing.T) {
	application := NewWithSources(slog.Default(), nil)
	application.Pool().Add(model.Proxy{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4})

	stats := application.CheckWithOptions(context.Background(), CheckOptions{
		Workers:         1,
		Profile:         scheduler.ProfileSmart,
		SkipUnsupported: true,
	})
	if stats.Scheduled != 0 || stats.SkippedUnsupported != 1 || stats.Unsupported != 0 {
		t.Fatalf("unexpected smart skip stats %#v", stats)
	}
	if stats.ByProtocol[model.ProtocolSOCKS4].SkippedUnsupported != 1 {
		t.Fatalf("expected by_protocol skip stats, got %#v", stats.ByProtocol)
	}
}

func TestCheckWithOptionsProtocolStats(t *testing.T) {
	application := NewWithSources(slog.Default(), nil)
	application.Pool().Add(model.Proxy{ID: "http://127.0.0.1:8080", Address: "127.0.0.1:8080", Protocol: model.ProtocolHTTP})
	application.Pool().Add(model.Proxy{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4})

	stats := application.CheckWithOptions(context.Background(), CheckOptions{
		Workers:   1,
		MaxChecks: 1,
	})
	if stats.Scheduled != 1 || stats.SkippedLimit != 1 {
		t.Fatalf("unexpected stats %#v", stats)
	}
	if stats.ByProtocol[model.ProtocolHTTP].Total != 1 || stats.ByProtocol[model.ProtocolSOCKS4].Total != 1 {
		t.Fatalf("unexpected protocol stats %#v", stats.ByProtocol)
	}
}

func TestFetchCheckWithOptionsChecksFastSourceBeforeSlowSourceCompletes(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("fast", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
		blockingSource{name: "slow", started: started, release: release},
	})
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	done := make(chan PipelineReport, 1)
	go func() {
		done <- application.FetchCheckWithOptions(context.Background(),
			FetchOptions{Workers: 2, CachePath: cachePath, CacheWrite: false},
			CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}},
		)
	}()

	<-started
	deadline := time.After(2 * time.Second)
	for {
		items := application.Pool().List(pool.Filter{Protocol: model.ProtocolSOCKS4})
		for _, item := range items {
			if item.ID == "socks4://127.0.0.1:1080" && item.CheckCount > 0 {
				close(release)
				report := <-done
				if report.Check.Unsupported != 1 || report.Fetch.Duplicates != 1 {
					t.Fatalf("expected duplicate slow proxy not to be rechecked, got fetch=%#v check=%#v", report.Fetch, report.Check)
				}
				return
			}
		}
		select {
		case <-deadline:
			close(release)
			t.Fatal("fast source proxy was not checked before slow source completed")
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestFetchCheckWithOptionsUsesCacheAndCheckTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	checkedAt := time.Now().Add(-time.Minute).UTC()
	if err := cache.Save(path, []model.Proxy{{
		ID:            "socks4://127.0.0.1:1080",
		Address:       "127.0.0.1:1080",
		Protocol:      model.ProtocolSOCKS4,
		HealthStatus:  model.HealthDead,
		HealthScore:   10,
		CheckCount:    1,
		LastCheckedAt: checkedAt,
	}}); err != nil {
		t.Fatal(err)
	}
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("duplicate", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
	})

	report := application.FetchCheckWithOptions(context.Background(),
		FetchOptions{Workers: 1, CachePath: path, CacheFallback: true, CacheWrite: true},
		CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}, CheckTTL: 30 * time.Minute, CachePath: path, CacheWrite: true},
	)
	if report.Check.Total != 1 || report.Check.Scheduled != 0 || report.Check.SkippedRecent != 1 {
		t.Fatalf("unexpected check stats %#v", report.Check)
	}
	if report.Fetch.Added != 1 || report.Fetch.Duplicates != 0 {
		t.Fatalf("unexpected fetch report %#v", report.Fetch)
	}
}

func TestFetchCheckWithOptionsSmartProfileSkipsUnsupported(t *testing.T) {
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("fresh", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
	})

	report := application.FetchCheckWithOptions(context.Background(),
		FetchOptions{Workers: 1, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		CheckOptions{Workers: 1, Profile: scheduler.ProfileSmart, SkipUnsupported: true},
	)
	if report.Check.Scheduled != 0 || report.Check.SkippedUnsupported != 1 {
		t.Fatalf("unexpected smart pipeline stats %#v", report.Check)
	}
}

func TestFetchCheckWithOptionsSharesMaxChecksAcrossCacheAndSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := cache.Save(path, []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}); err != nil {
		t.Fatal(err)
	}
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("fresh", []model.Proxy{{ID: "socks4://127.0.0.2:1080", Address: "127.0.0.2:1080", Protocol: model.ProtocolSOCKS4}}),
	})

	report := application.FetchCheckWithOptions(context.Background(),
		FetchOptions{Workers: 1, CachePath: path, CacheFallback: true, CacheWrite: false},
		CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}, MaxChecks: 1},
	)
	if report.Check.Total != 2 || report.Check.Scheduled != 1 || report.Check.SkippedLimit != 1 {
		t.Fatalf("unexpected check stats %#v", report.Check)
	}
}

func TestFetchCheckWithOptionsDeduplicatesSourceProxies(t *testing.T) {
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("one", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
		source.NewStatic("two", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
	})

	report := application.FetchCheckWithOptions(context.Background(),
		FetchOptions{Workers: 2, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}},
	)
	if report.Fetch.Duplicates != 1 || report.Fetch.Added != 1 {
		t.Fatalf("unexpected fetch report %#v", report.Fetch)
	}
	if report.Check.Scheduled != 1 || report.Check.Unsupported != 1 {
		t.Fatalf("unexpected check stats %#v", report.Check)
	}
	metrics := application.Metrics(RefreshOptions{})
	if metrics.Fetch.Added != 1 || metrics.Check.Scheduled != 1 || metrics.Pool.Total != 1 {
		t.Fatalf("expected pipeline metrics, got %#v", metrics)
	}
}

func TestFetchCheckWithOptionsUsesCacheWhenSourcesFail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.json")
	if err := cache.Save(path, []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}); err != nil {
		t.Fatal(err)
	}
	application := NewWithSources(slog.Default(), []source.Source{failingSource{name: "fail"}})

	report := application.FetchCheckWithOptions(context.Background(),
		FetchOptions{Workers: 1, CachePath: path, CacheFallback: true, CacheWrite: true},
		CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}, CachePath: path, CacheWrite: true},
	)
	if !report.Fetch.ReusedFromCache || report.Fetch.Added != 1 || report.Fetch.FailedSources != 1 {
		t.Fatalf("unexpected fetch report %#v", report.Fetch)
	}
	if report.Check.Scheduled != 1 || report.Check.Unsupported != 1 {
		t.Fatalf("unexpected check stats %#v", report.Check)
	}
}

func TestFetchWithOptionsSkipsSourceDuringCooldown(t *testing.T) {
	application := NewWithSources(slog.Default(), []source.Source{failingSource{name: "fail"}})
	options := FetchOptions{
		Workers:                1,
		CachePath:              filepath.Join(t.TempDir(), "cache.json"),
		CacheFallback:          false,
		CacheWrite:             false,
		SourceFailureThreshold: 1,
		SourceCooldown:         time.Hour,
	}

	first := application.FetchWithOptions(context.Background(), options)
	if first.FailedSources != 1 || first.SkippedSources != 0 {
		t.Fatalf("unexpected first report %#v", first)
	}

	second := application.FetchWithOptions(context.Background(), options)
	if second.FailedSources != 0 || second.SkippedSources != 1 || second.Sources[0].Status != "skipped_cooldown" {
		t.Fatalf("unexpected cooldown report %#v", second)
	}
}

func TestFetchWithOptionsUsesSourceAfterCooldownExpires(t *testing.T) {
	application := NewWithSources(slog.Default(), []source.Source{failingSource{name: "fail"}})
	options := FetchOptions{
		Workers:                1,
		CachePath:              filepath.Join(t.TempDir(), "cache.json"),
		CacheFallback:          false,
		CacheWrite:             false,
		SourceFailureThreshold: 1,
		SourceCooldown:         time.Millisecond,
	}

	application.FetchWithOptions(context.Background(), options)
	time.Sleep(5 * time.Millisecond)
	report := application.FetchWithOptions(context.Background(), options)
	if report.FailedSources != 1 || report.SkippedSources != 0 {
		t.Fatalf("expected source to run after cooldown, got %#v", report)
	}
}

func TestTriggerRefreshSkipsWhenAlreadyRunning(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	application := NewWithSources(slog.Default(), []source.Source{blockingSource{name: "block", started: started, release: release}})
	options := RefreshOptions{
		Fetch: FetchOptions{Workers: 1, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		Check: CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}},
	}

	first := application.TriggerRefresh(context.Background(), options)
	if !first.Running {
		t.Fatalf("expected first refresh to be running")
	}
	<-started
	second := application.TriggerRefresh(context.Background(), options)
	if second.Status != "skipped" || second.Running || second.SkippedAt.IsZero() || second.SkippedReason != "already_running" {
		t.Fatalf("expected second refresh to be skipped while running, got %#v", second)
	}

	close(release)
	deadline := time.After(2 * time.Second)
	for {
		status := application.RefreshStatus()
		if status.Status == "completed" {
			if status.Pipeline.Check.Scheduled == 0 || status.Check.Scheduled == 0 {
				t.Fatalf("expected refresh pipeline report, got %#v", status)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("refresh did not complete, last status %#v", status)
		case <-time.After(10 * time.Millisecond):
		}
	}
}
