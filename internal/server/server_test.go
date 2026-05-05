package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LING71671/plugproxy/internal/pool"
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
