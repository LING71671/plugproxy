package model

import (
	"testing"
	"time"
)

func TestApplyCheckSuccessBoostsFastProxyToHealthy(t *testing.T) {
	proxy := Proxy{Address: "1.1.1.1:8080", Protocol: ProtocolHTTP}
	proxy = ApplyCheck(proxy, CheckUpdate{OK: true, Latency: 500 * time.Millisecond})

	if proxy.HealthScore != 75 {
		t.Fatalf("expected score 75, got %d", proxy.HealthScore)
	}
	if proxy.HealthStatus != HealthHealthy {
		t.Fatalf("expected healthy, got %s", proxy.HealthStatus)
	}
}

func TestApplyCheckConsecutiveFailuresBecomeDead(t *testing.T) {
	proxy := Proxy{Address: "1.1.1.1:8080", Protocol: ProtocolHTTP}
	for range 3 {
		proxy = ApplyCheck(proxy, CheckUpdate{Error: "boom"})
	}

	if proxy.HealthStatus != HealthDead {
		t.Fatalf("expected dead, got %s", proxy.HealthStatus)
	}
	if proxy.ConsecutiveFailures != 3 {
		t.Fatalf("expected 3 failures, got %d", proxy.ConsecutiveFailures)
	}
}
