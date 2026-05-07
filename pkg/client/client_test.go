package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestClientGetProxyBuildsQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxy" {
			t.Fatalf("expected /proxy, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("strategy") != "fastest" {
			t.Fatalf("missing strategy query")
		}
		if r.URL.Query().Get("protocol") != "http" {
			t.Fatalf("missing protocol query")
		}
		if r.URL.Query().Get("healthy") != "true" {
			t.Fatalf("missing healthy query")
		}
		if r.URL.Query().Get("exclude_dead") != "true" {
			t.Fatalf("missing exclude_dead query")
		}
		_ = json.NewEncoder(w).Encode(model.Proxy{ID: "http://a:1", Address: "a:1", Protocol: model.ProtocolHTTP})
	}))
	defer server.Close()

	proxy, err := New(server.URL).GetProxy(context.Background(), GetProxyOptions{
		Strategy:    "fastest",
		Protocol:    model.ProtocolHTTP,
		Healthy:     true,
		ExcludeDead: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if proxy.ID != "http://a:1" {
		t.Fatalf("unexpected proxy %#v", proxy)
	}
}

func TestClientListProxiesBuildsQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/proxies" {
			t.Fatalf("expected /proxies, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("status") != "healthy" || r.URL.Query().Get("source") != "one" {
			t.Fatalf("unexpected query %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("offset") != "5" {
			t.Fatalf("unexpected pagination %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("exclude_dead") != "true" {
			t.Fatalf("missing exclude_dead query")
		}
		_ = json.NewEncoder(w).Encode([]model.Proxy{{ID: "http://a:1"}})
	}))
	defer server.Close()

	proxies, err := New(server.URL).ListProxies(context.Background(), ListOptions{
		Status:      model.HealthHealthy,
		Source:      "one",
		ExcludeDead: true,
		Limit:       10,
		Offset:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
}

func TestClientStatsAndRefresh(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/stats":
			_ = json.NewEncoder(w).Encode(model.ProxyStats{Total: 2})
		case "/metrics.json":
			_ = json.NewEncoder(w).Encode(map[string]any{"uptime_ms": 123})
		case "/refresh":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "running"})
		case "/refresh/cancel":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "cancelling"})
		case "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case "/readyz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ready"})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(server.URL)
	stats, err := c.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 2 {
		t.Fatalf("expected total 2, got %d", stats.Total)
	}

	metrics, err := c.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if metrics["uptime_ms"] != float64(123) {
		t.Fatalf("unexpected metrics %#v", metrics)
	}

	refresh, err := c.TriggerRefresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if refresh["status"] != "running" {
		t.Fatalf("unexpected refresh status %#v", refresh)
	}

	cancelled, err := c.CancelRefresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cancelled["status"] != "cancelling" {
		t.Fatalf("unexpected cancel status %#v", cancelled)
	}

	health, err := c.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if health["status"] != "ok" {
		t.Fatalf("unexpected health status %#v", health)
	}

	ready, err := c.Ready(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ready["status"] != "ready" {
		t.Fatalf("unexpected ready status %#v", ready)
	}
}

func TestClientReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no proxy", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := New(server.URL).GetProxy(context.Background(), GetProxyOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	var httpErr Error
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTP error, got %T", err)
	}
	if httpErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", httpErr.StatusCode)
	}
}
