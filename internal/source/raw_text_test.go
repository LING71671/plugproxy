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

func TestParseHTMLTextProxiesExtractsBRSeparatedItems(t *testing.T) {
	content := `<html><body><script>var x = 1;</script>
		200.55.254.1:6969<br>190.82.91.204:999<br>
		<a href="/free">not a proxy</a>
		socks5://8.8.8.8:1080<br>999.1.1.1:80<br>1.2.3.4:70000
	</body></html>`

	proxies := ParseHTMLTextProxies(content, model.ProtocolHTTP, "html")
	if len(proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d: %#v", len(proxies), proxies)
	}
	if proxies[0].Protocol != model.ProtocolHTTP || proxies[0].Address != "200.55.254.1:6969" {
		t.Fatalf("unexpected first proxy %#v", proxies[0])
	}
	if proxies[2].Protocol != model.ProtocolSOCKS5 || proxies[2].Address != "8.8.8.8:1080" {
		t.Fatalf("unexpected protocol proxy %#v", proxies[2])
	}
}

func TestParseHTMLTextProxiesExtractsTableCells(t *testing.T) {
	content := `<table><tbody>
		<tr><td>221.174.219.150</td><td>8088</td><td>HTTP</td><td>江苏省宿迁市</td></tr>
		<tr><td>8.8.8.8</td><td>1080</td><td>SOCKS5</td><td>US</td></tr>
	</tbody></table>`

	proxies := ParseHTMLTextProxies(content, model.ProtocolHTTP, "table")
	if len(proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d: %#v", len(proxies), proxies)
	}
	if proxies[0].Protocol != model.ProtocolHTTP || proxies[0].Address != "221.174.219.150:8088" {
		t.Fatalf("unexpected first proxy %#v", proxies[0])
	}
	if proxies[1].Protocol != model.ProtocolSOCKS5 || proxies[1].Address != "8.8.8.8:1080" {
		t.Fatalf("unexpected second proxy %#v", proxies[1])
	}
}
