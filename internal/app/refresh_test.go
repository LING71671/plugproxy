package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestDecideRefreshHealthyBelowMin(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	decision := DecideRefresh(now, model.ProxyStats{Total: 10, Healthy: 1}, RefreshStatus{}, RefreshPolicy{
		Enabled:      true,
		BaseInterval: time.Minute,
		MinInterval:  time.Second,
		MaxInterval:  time.Hour,
		MinHealthy:   2,
	})
	if decision.Delay != 0 || decision.Reason != "healthy_below_min" {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestDecideRefreshHealthyRatioBelowMin(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	decision := DecideRefresh(now, model.ProxyStats{Total: 10, Healthy: 2}, RefreshStatus{}, RefreshPolicy{
		Enabled:         true,
		BaseInterval:    time.Minute,
		MinInterval:     time.Second,
		MaxInterval:     time.Hour,
		MinHealthyRatio: 0.5,
	})
	if decision.Delay != 0 || decision.Reason != "healthy_ratio_below_min" {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestDecideRefreshUncheckedAboveThreshold(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	decision := DecideRefresh(now, model.ProxyStats{Total: 10, Healthy: 10, Unchecked: 3}, RefreshStatus{}, RefreshPolicy{
		Enabled:            true,
		BaseInterval:       time.Minute,
		MinInterval:        time.Second,
		MaxInterval:        time.Hour,
		MinHealthy:         1,
		UncheckedThreshold: 3,
	})
	if decision.Delay != 0 || decision.Reason != "unchecked_above_threshold" {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestDecideRefreshIdleClampsDelay(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	decision := DecideRefresh(now, model.ProxyStats{Total: 10, Healthy: 10}, RefreshStatus{}, RefreshPolicy{
		Enabled:            true,
		BaseInterval:       5 * time.Second,
		MinInterval:        10 * time.Second,
		MaxInterval:        time.Hour,
		Jitter:             0,
		MinHealthy:         1,
		UncheckedThreshold: 100,
	})
	if decision.Delay != 10*time.Second || decision.Reason != "idle" {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestDecideRefreshBackoffClampsToMax(t *testing.T) {
	now := time.Date(2026, 5, 6, 10, 0, 0, 0, time.UTC)
	decision := DecideRefresh(now, model.ProxyStats{Total: 10, Healthy: 10}, RefreshStatus{
		Fetch: FetchReport{FailedSources: 1},
	}, RefreshPolicy{
		Enabled:            true,
		BaseInterval:       10 * time.Minute,
		MinInterval:        time.Second,
		MaxInterval:        15 * time.Minute,
		Jitter:             0,
		MinHealthy:         1,
		UncheckedThreshold: 100,
		FailureBackoff:     2,
	})
	if decision.Delay != 15*time.Minute || decision.Reason != "backoff" {
		t.Fatalf("unexpected decision %#v", decision)
	}
}

func TestStartAutoRefreshTriggersWhenHealthyBelowMin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("one", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
	})
	options := RefreshOptions{
		Fetch: FetchOptions{Workers: 1, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		Check: CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}},
		Policy: RefreshPolicy{
			Enabled:            true,
			BaseInterval:       time.Hour,
			MinInterval:        10 * time.Millisecond,
			MaxInterval:        time.Hour,
			Jitter:             0,
			MinHealthy:         1,
			UncheckedThreshold: 100,
		},
	}

	application.StartAutoRefresh(ctx, time.Hour, options)
	deadline := time.After(2 * time.Second)
	for {
		status := application.RefreshStatus()
		if status.Status == "completed" {
			if status.LastReason != "healthy_below_min" || status.Pipeline.Check.Scheduled == 0 {
				t.Fatalf("unexpected refresh status %#v", status)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("auto refresh did not complete, last status %#v", application.RefreshStatus())
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestStartAutoRefreshDisabledPolicyDoesNotStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	release := make(chan struct{})
	application := NewWithSources(slog.Default(), []source.Source{blockingSource{name: "block", started: started, release: release}})
	application.StartAutoRefresh(ctx, time.Millisecond, RefreshOptions{
		Fetch: FetchOptions{Workers: 1, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		Check: CheckOptions{Workers: 1},
		Policy: RefreshPolicy{
			Enabled:      false,
			BaseInterval: time.Millisecond,
		},
	})

	select {
	case <-started:
		close(release)
		t.Fatal("expected disabled auto refresh not to start")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTriggerRefreshMarksManualReason(t *testing.T) {
	application := NewWithSources(slog.Default(), []source.Source{
		source.NewStatic("one", []model.Proxy{{ID: "socks4://127.0.0.1:1080", Address: "127.0.0.1:1080", Protocol: model.ProtocolSOCKS4}}),
	})
	status := application.TriggerRefresh(context.Background(), RefreshOptions{
		Fetch: FetchOptions{Workers: 1, CachePath: filepath.Join(t.TempDir(), "cache.json"), CacheWrite: false},
		Check: CheckOptions{Workers: 1, Filter: pool.Filter{Protocol: model.ProtocolSOCKS4}},
	})
	if status.LastReason != "manual" {
		t.Fatalf("expected manual reason, got %#v", status)
	}
}
