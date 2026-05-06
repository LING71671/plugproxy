package model

import "testing"

func TestNewProxyStats(t *testing.T) {
	stats := NewProxyStats([]Proxy{
		{Protocol: ProtocolHTTP, Source: "a", HealthStatus: HealthHealthy},
		{Protocol: ProtocolHTTP, Source: "a", HealthStatus: HealthDead},
		{Protocol: ProtocolSOCKS5, Source: "b", HealthStatus: HealthUnchecked},
	})

	if stats.Total != 3 {
		t.Fatalf("expected total 3, got %d", stats.Total)
	}
	if stats.Healthy != 1 || stats.Dead != 1 || stats.Unchecked != 1 {
		t.Fatalf("unexpected status counts %#v", stats)
	}
	if stats.Protocols[ProtocolHTTP] != 2 || stats.Protocols[ProtocolSOCKS5] != 1 {
		t.Fatalf("unexpected protocol counts %#v", stats.Protocols)
	}
	if stats.Sources["a"] != 2 || stats.Sources["b"] != 1 {
		t.Fatalf("unexpected source counts %#v", stats.Sources)
	}
}
