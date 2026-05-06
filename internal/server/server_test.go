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
	srv := New(pool.NewMemory(), slog.Default()).WithRefresh(func(context.Context) any {
		triggered = true
		return map[string]any{"status": "running", "running": true}
	}, func() any {
		return map[string]any{"status": "idle", "running": false}
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
