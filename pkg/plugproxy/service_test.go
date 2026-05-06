package plugproxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LING71671/plugproxy/pkg/client"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestServiceStartGetProxyAndClose(t *testing.T) {
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "127.0.0.1:8080")
	}))
	defer sourceServer.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "sources.json")
	cachePath := filepath.Join(dir, "cache.json")
	data := fmt.Sprintf(`{"sources":[{"name":"test","type":"raw_text_url","url":%q,"protocol_hint":"http","enabled":true}]}`, sourceServer.URL)
	if err := os.WriteFile(configPath, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := New(Config{
		Addr:       "127.0.0.1:0",
		ConfigPath: configPath,
		CachePath:  cachePath,
		SkipCheck:  true,
		Refresh:    false,
	})
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := svc.Close(closeCtx); err != nil {
			t.Fatal(err)
		}
	}()

	if svc.URL() == "" {
		t.Fatal("expected service URL")
	}
	if svc.Client() == nil {
		t.Fatal("expected service client")
	}

	proxy, err := svc.GetProxy(ctx, client.GetProxyOptions{Protocol: model.ProtocolHTTP})
	if err != nil {
		t.Fatal(err)
	}
	if proxy.Address != "127.0.0.1:8080" {
		t.Fatalf("unexpected proxy %#v", proxy)
	}

	proxies, err := svc.ListProxies(ctx, client.ListOptions{Protocol: model.ProtocolHTTP})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}
}

func TestServiceRefreshRequiresStart(t *testing.T) {
	svc := New(Config{})
	_, err := svc.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
