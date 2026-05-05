package source

import (
	"testing"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestParseProxyLineWithProtocol(t *testing.T) {
	proxy, ok := ParseProxyLine("socks5://1.2.3.4:1080", model.ProtocolHTTP)
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.Protocol != model.ProtocolSOCKS5 {
		t.Fatalf("expected socks5, got %s", proxy.Protocol)
	}
	if proxy.Address != "1.2.3.4:1080" {
		t.Fatalf("unexpected address %s", proxy.Address)
	}
}

func TestParseProxyLineUsesProtocolHint(t *testing.T) {
	proxy, ok := ParseProxyLine("1.2.3.4:8080", model.ProtocolHTTPS)
	if !ok {
		t.Fatal("expected proxy")
	}
	if proxy.Protocol != model.ProtocolHTTPS {
		t.Fatalf("expected https, got %s", proxy.Protocol)
	}
}

func TestParseRawTextProxiesSkipsInvalidAndDeduplicates(t *testing.T) {
	content := `
		1.2.3.4:8080
		invalid
		http://1.2.3.4:8080
		socks4://5.6.7.8:1080
	`
	proxies := ParseRawTextProxies(content, model.ProtocolHTTP, "test")
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d: %#v", len(proxies), proxies)
	}
	if proxies[0].Source != "test" {
		t.Fatalf("expected source name set")
	}
}
