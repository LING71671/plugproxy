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
