package source

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LING71671/plugproxy/pkg/model"
)

func TestParseJSONProxiesStringArrayIPPort(t *testing.T) {
	proxies, err := ParseJSONProxies([]byte(`["1.1.1.1:8080"]`), model.ProtocolHTTP, "json", JSONConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].ID != "http://1.1.1.1:8080" {
		t.Fatalf("unexpected proxies %#v", proxies)
	}
}

func TestParseJSONProxiesStringArrayWithProtocol(t *testing.T) {
	proxies, err := ParseJSONProxies([]byte(`["socks5://2.2.2.2:1080"]`), model.ProtocolHTTP, "json", JSONConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].Protocol != model.ProtocolSOCKS5 {
		t.Fatalf("unexpected proxies %#v", proxies)
	}
}

func TestParseJSONProxiesObjectArrayHostPortProtocol(t *testing.T) {
	data := []byte(`[{"ip":"1.1.1.1","port":8080,"protocol":"https"}]`)
	proxies, err := ParseJSONProxies(data, "", "json", JSONConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].ID != "https://1.1.1.1:8080" {
		t.Fatalf("unexpected proxies %#v", proxies)
	}
}

func TestParseJSONProxiesObjectProxyFields(t *testing.T) {
	cases := []string{"proxy", "url", "address", "addr", "ip", "host"}
	for _, field := range cases {
		data := []byte(`[{"` + field + `":"http://1.1.1.1:8080"}]`)
		proxies, err := ParseJSONProxies(data, "", "json", JSONConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if len(proxies) != 1 || proxies[0].Address != "1.1.1.1:8080" {
			t.Fatalf("%s: unexpected proxies %#v", field, proxies)
		}
	}
}

func TestParseJSONProxiesRootObjectArrays(t *testing.T) {
	cases := []string{"proxies", "data", "items", "results"}
	for _, key := range cases {
		data := []byte(`{"` + key + `":["1.1.1.1:8080"]}`)
		proxies, err := ParseJSONProxies(data, model.ProtocolHTTP, "json", JSONConfig{})
		if err != nil {
			t.Fatal(err)
		}
		if len(proxies) != 1 {
			t.Fatalf("%s: expected 1 proxy, got %#v", key, proxies)
		}
	}
}

func TestParseJSONProxiesCustomMapping(t *testing.T) {
	data := []byte(`{"rows":[{"server":"1.1.1.1","listen":"8080","kind":"socks4"}]}`)
	proxies, err := ParseJSONProxies(data, model.ProtocolHTTP, "json", JSONConfig{
		ItemsPath:     "rows",
		HostField:     "server",
		PortField:     "listen",
		ProtocolField: "kind",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].ID != "socks4://1.1.1.1:8080" {
		t.Fatalf("unexpected proxies %#v", proxies)
	}
}

func TestParseJSONProxiesProtocolFallbackSkipsInvalidAndDeduplicates(t *testing.T) {
	data := []byte(`[
		{"ip":"1.1.1.1","port":8080},
		{"ip":"bad host","port":8080},
		{"proxy":"1.1.1.1:8080"}
	]`)
	proxies, err := ParseJSONProxies(data, model.ProtocolHTTPS, "json", JSONConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 || proxies[0].ID != "https://1.1.1.1:8080" {
		t.Fatalf("unexpected proxies %#v", proxies)
	}
}

func TestJSONURLSourceSendsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("expected Accept header, got %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "custom-agent" {
			t.Fatalf("expected custom User-Agent, got %q", got)
		}
		_, _ = w.Write([]byte(`["1.1.1.1:8080"]`))
	}))
	defer server.Close()

	source := NewJSONURL(JSONURLOption{
		Name:         "api",
		URL:          server.URL,
		ProtocolHint: model.ProtocolHTTP,
		Headers: map[string]string{
			"Accept":     "application/json",
			"User-Agent": "custom-agent",
		},
	})
	proxies, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %#v", proxies)
	}
}
