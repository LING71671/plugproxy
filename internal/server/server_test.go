package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestRefreshEndpoints(t *testing.T) {
	triggered := false
	cancelled := false
	srv := New(pool.NewMemory(), slog.Default()).WithRefresh(func(context.Context) any {
		triggered = true
		return map[string]any{"status": "running", "running": true}
	}, func() any {
		return map[string]any{"status": "idle", "running": false}
	}, func() any {
		cancelled = true
		return map[string]any{"status": "cancelling", "running": true}
	})

	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	getResp, err := http.Get(server.URL + "/refresh")
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	var getBody map[string]any
	if err := json.NewDecoder(getResp.Body).Decode(&getBody); err != nil {
		t.Fatal(err)
	}
	if getBody["status"] != "idle" {
		t.Fatalf("expected idle status, got %#v", getBody)
	}

	postResp, err := http.Post(server.URL+"/refresh", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", postResp.StatusCode)
	}
	if !triggered {
		t.Fatal("expected refresh trigger")
	}

	cancelResp, err := http.Post(server.URL+"/refresh/cancel", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelResp.Body.Close()
	if cancelResp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", cancelResp.StatusCode)
	}
	if !cancelled {
		t.Fatal("expected refresh cancel")
	}
}

func TestStatsEndpoint(t *testing.T) {
	proxyPool := pool.NewMemory()
	proxyPool.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, Source: "one", HealthStatus: model.HealthHealthy})
	proxyPool.Add(model.Proxy{ID: "socks5://b:1", Address: "b:1", Protocol: model.ProtocolSOCKS5, Source: "two", HealthStatus: model.HealthDead})
	srv := New(proxyPool, slog.Default())

	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var stats model.ProxyStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 || stats.Healthy != 1 || stats.Dead != 1 {
		t.Fatalf("unexpected stats %#v", stats)
	}
	if stats.Protocols[model.ProtocolHTTP] != 1 || stats.Sources["one"] != 1 {
		t.Fatalf("unexpected distributions %#v", stats)
	}
}

func TestMetricsAndUIEndpoints(t *testing.T) {
	srv := New(pool.NewMemory(), slog.Default()).WithMetrics(func() any {
		return map[string]any{"uptime_ms": 123}
	})
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	metricsResp, err := http.Get(server.URL + "/metrics.json")
	if err != nil {
		t.Fatal(err)
	}
	defer metricsResp.Body.Close()
	var metrics map[string]any
	if err := json.NewDecoder(metricsResp.Body).Decode(&metrics); err != nil {
		t.Fatal(err)
	}
	if metrics["uptime_ms"] != float64(123) {
		t.Fatalf("unexpected metrics %#v", metrics)
	}

	uiResp, err := http.Get(server.URL + "/ui/")
	if err != nil {
		t.Fatal(err)
	}
	defer uiResp.Body.Close()
	if uiResp.StatusCode != http.StatusOK {
		t.Fatalf("expected ui 200, got %d", uiResp.StatusCode)
	}
}

func TestHealthzAndReadyzEndpoints(t *testing.T) {
	proxyPool := pool.NewMemory()
	srv := New(proxyPool, slog.Default())
	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	healthResp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("expected healthz 200, got %d", healthResp.StatusCode)
	}

	notReadyResp, err := http.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer notReadyResp.Body.Close()
	if notReadyResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected readyz 503 without proxies, got %d", notReadyResp.StatusCode)
	}
	var errBody map[string]string
	if err := json.NewDecoder(notReadyResp.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	if errBody["error_type"] != "no_proxy_available" {
		t.Fatalf("expected unified error response, got %#v", errBody)
	}

	proxyPool.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded})
	readyResp, err := http.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer readyResp.Body.Close()
	if readyResp.StatusCode != http.StatusOK {
		t.Fatalf("expected readyz 200, got %d", readyResp.StatusCode)
	}
}

func TestListProxiesFiltersAndPaginates(t *testing.T) {
	proxyPool := pool.NewMemory()
	proxyPool.Add(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP, Source: "one", HealthStatus: model.HealthHealthy})
	proxyPool.Add(model.Proxy{ID: "http://b:1", Address: "b:1", Protocol: model.ProtocolHTTP, Source: "one", HealthStatus: model.HealthHealthy})
	proxyPool.Add(model.Proxy{ID: "http://c:1", Address: "c:1", Protocol: model.ProtocolHTTP, Source: "two", HealthStatus: model.HealthDead})
	srv := New(proxyPool, slog.Default())

	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/proxies?status=healthy&source=one&limit=1&offset=1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var proxies []model.Proxy
	if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
	if proxies[0].Address != "b:1" {
		t.Fatalf("expected b:1, got %s", proxies[0].Address)
	}
}

func TestGetProxyExcludesDeadByDefault(t *testing.T) {
	proxyPool := pool.NewMemory()
	proxyPool.Add(model.Proxy{ID: "http://dead:1", Address: "dead:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDead})
	proxyPool.Add(model.Proxy{ID: "http://degraded:1", Address: "degraded:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded})
	srv := New(proxyPool, slog.Default())

	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/proxy")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var proxy model.Proxy
	if err := json.NewDecoder(resp.Body).Decode(&proxy); err != nil {
		t.Fatal(err)
	}
	if proxy.Address != "degraded:1" {
		t.Fatalf("expected degraded proxy, got %#v", proxy)
	}
}

func TestListProxiesCanExcludeDead(t *testing.T) {
	proxyPool := pool.NewMemory()
	proxyPool.Add(model.Proxy{ID: "http://dead:1", Address: "dead:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDead})
	proxyPool.Add(model.Proxy{ID: "http://degraded:1", Address: "degraded:1", Protocol: model.ProtocolHTTP, HealthStatus: model.HealthDegraded})
	srv := New(proxyPool, slog.Default())

	server := httptest.NewServer(srv.Handler())
	defer server.Close()

	resp, err := http.Get(server.URL + "/proxies?exclude_dead=true")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var proxies []model.Proxy
	if err := json.NewDecoder(resp.Body).Decode(&proxies); err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].Address != "degraded:1" {
		t.Fatalf("expected degraded proxy only, got %#v", proxies)
	}
}
