package pool

import (
	"testing"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestMemoryPoolHealthyFilterUsesHealthStatus(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy})
	p.Add(model.Proxy{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded})

	items := p.List(Filter{Healthy: true})
	if len(items) != 1 {
		t.Fatalf("expected 1 healthy proxy, got %d", len(items))
	}
	if items[0].Address != "a:1" {
		t.Fatalf("expected healthy proxy first, got %s", items[0].Address)
	}
}

func TestMemoryPoolDefaultsHealthStatusToUnchecked(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP})

	items := p.List(Filter{})
	if len(items) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(items))
	}
	if items[0].HealthStatus != model.HealthUnchecked {
		t.Fatalf("expected unchecked, got %s", items[0].HealthStatus)
	}
	if items[0].HealthScore != 50 {
		t.Fatalf("expected initial health score 50, got %d", items[0].HealthScore)
	}
}

func TestMemoryPoolFastestPrefersHealthyAndLatency(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://slow:1", Address: "slow:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 90, Latency: 2 * time.Second})
	p.Add(model.Proxy{ID: "http://fast:1", Address: "fast:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 80, Latency: 100 * time.Millisecond})
	p.Add(model.Proxy{ID: "http://dead:1", Address: "dead:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDead, HealthScore: 100, Latency: 10 * time.Millisecond})

	proxy, ok := p.Get(StrategyFastest, Filter{})
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.Address != "fast:1" {
		t.Fatalf("expected fastest healthy proxy, got %s", proxy.Address)
	}
}

func TestMemoryPoolPreservesHealthWhenFetchedAgain(t *testing.T) {
	p := NewMemory()
	checkedAt := time.Now().Add(-time.Minute)
	p.Add(model.Proxy{
		ID:            "http://a:1",
		Address:       "a:1",
		Protocol:      model.ProtocolHTTP,
		HealthStatus:  model.HealthHealthy,
		HealthScore:   90,
		CheckCount:    2,
		LastCheckedAt: checkedAt,
		Source:        "cache",
	})

	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, Source: "fresh"})

	items := p.List(Filter{})
	if len(items) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(items))
	}
	if items[0].Source != "fresh" {
		t.Fatalf("expected source refresh, got %s", items[0].Source)
	}
	if items[0].HealthStatus != model.HealthHealthy || items[0].HealthScore != 90 {
		t.Fatalf("expected health to be preserved, got %s/%d", items[0].HealthStatus, items[0].HealthScore)
	}
	if !items[0].LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected last checked to be preserved")
	}
}

func TestMemoryPoolListFiltersStatusAndSource(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, Source: "one", HealthStatus: model.HealthHealthy})
	p.Add(model.Proxy{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, Source: "two", HealthStatus: model.HealthDead})

	items := p.List(Filter{Status: model.HealthHealthy, Source: "one"})
	if len(items) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(items))
	}
	if items[0].Address != "a:1" {
		t.Fatalf("expected a:1, got %s", items[0].Address)
	}
}
