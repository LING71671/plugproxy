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

func TestMemoryPoolGetUpdatesUsage(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 90})

	proxy, ok := p.Get(StrategyAny, Filter{})
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.UseCount != 1 || proxy.LastUsedAt.IsZero() {
		t.Fatalf("expected usage metadata to be updated, got %#v", proxy)
	}

	items := p.List(Filter{})
	if items[0].UseCount != 1 || items[0].LastUsedAt.IsZero() {
		t.Fatalf("expected usage metadata to be persisted, got %#v", items[0])
	}
}

func TestMemoryPoolLeastRecentlyUsedPrefersUnused(t *testing.T) {
	p := NewMemory()
	now := time.Now()
	p.Add(model.Proxy{ID: "http://used:1", Address: "used:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 90, LastUsedAt: now.Add(-time.Minute), UseCount: 1})
	p.Add(model.Proxy{ID: "http://unused:1", Address: "unused:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 80})

	proxy, ok := p.Get(StrategyLeastRecentlyUsed, Filter{})
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.Address != "unused:1" {
		t.Fatalf("expected unused proxy, got %s", proxy.Address)
	}
}

func TestMemoryPoolRoundRobinCyclesWithinFilter(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 90})
	p.Add(model.Proxy{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 80})

	first, ok := p.Get(StrategyRoundRobin, Filter{})
	if !ok {
		t.Fatal("expected first proxy")
	}
	second, ok := p.Get(StrategyRoundRobin, Filter{})
	if !ok {
		t.Fatal("expected second proxy")
	}
	third, ok := p.Get(StrategyRoundRobin, Filter{})
	if !ok {
		t.Fatal("expected third proxy")
	}

	if first.Address == second.Address {
		t.Fatalf("expected round robin to cycle, got %s twice", first.Address)
	}
	if third.Address != first.Address {
		t.Fatalf("expected round robin to wrap to %s, got %s", first.Address, third.Address)
	}
}

func TestMemoryPoolWeightedUsesHealthSeenAndUsage(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://low:1", Address: "low:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 70, SeenCount: 1, UseCount: 10})
	p.Add(model.Proxy{ID: "http://high:1", Address: "high:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthHealthy, HealthScore: 90, SeenCount: 5})

	proxy, ok := p.Get(StrategyWeighted, Filter{})
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.Address != "high:1" {
		t.Fatalf("expected higher weighted proxy, got %s", proxy.Address)
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

	p.AddSeen(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, Source: "fresh"})

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
	if items[0].SeenCount != 1 || items[0].LastSeenAt.IsZero() {
		t.Fatalf("expected seen metadata to be updated, got %#v", items[0])
	}
}

func TestMemoryPoolPreservesUsageWhenFetchedOrCheckedAgain(t *testing.T) {
	p := NewMemory()
	usedAt := time.Now().Add(-time.Minute)
	p.Add(model.Proxy{
		ID:            "http://a:1",
		Address:       "a:1",
		Protocol:      model.ProtocolHTTP,
		HealthStatus:  model.HealthHealthy,
		HealthScore:   90,
		CheckCount:    1,
		LastCheckedAt: time.Now().Add(-2 * time.Minute),
		LastUsedAt:    usedAt,
		UseCount:      2,
	})

	p.AddSeen(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP})
	item := p.List(Filter{})[0]
	if !item.LastUsedAt.Equal(usedAt) || item.UseCount != 2 {
		t.Fatalf("expected usage metadata to survive fetch merge, got %#v", item)
	}

	item.LastCheckedAt = time.Now()
	item.CheckCount++
	item.HealthScore = 100
	p.Add(item)
	checked := p.List(Filter{})[0]
	if !checked.LastUsedAt.Equal(usedAt) || checked.UseCount != 2 {
		t.Fatalf("expected usage metadata to survive check merge, got %#v", checked)
	}
}

func TestMemoryPoolDoesNotIncrementSeenOnCheckResult(t *testing.T) {
	p := NewMemory()
	p.AddSeen(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP})
	item := p.List(Filter{})[0]

	item.CheckCount = 1
	item.LastCheckedAt = time.Now()
	item.HealthStatus = model.HealthDegraded
	p.Add(item)

	items := p.List(Filter{})
	if items[0].SeenCount != 1 {
		t.Fatalf("expected check result not to increment seen count, got %#v", items[0])
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

func TestMemoryPoolExcludeDeadFilter(t *testing.T) {
	p := NewMemory()
	p.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded})
	p.Add(model.Proxy{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDead})

	items := p.List(Filter{ExcludeDead: true})
	if len(items) != 1 {
		t.Fatalf("expected 1 non-dead proxy, got %d", len(items))
	}
	if items[0].Address != "a:1" {
		t.Fatalf("expected a:1, got %s", items[0].Address)
	}
}
