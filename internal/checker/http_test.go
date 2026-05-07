package checker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LING71671/plugproxy/internal/errtype"
	"github.com/LING71671/plugproxy/pkg/model"
)

func TestHTTPCheckerSOCKS4Unsupported(t *testing.T) {
	result := NewHTTP("", 0).Check(context.Background(), model.Proxy{
		Address:  "127.0.0.1:1080",
		Protocol: model.ProtocolSOCKS4,
	})

	if !result.Unsupported {
		t.Fatal("expected unsupported SOCKS4 result")
	}
	if result.OK {
		t.Fatal("unsupported result must not be OK")
	}
}

func TestHTTPCheckerTransportDefaults(t *testing.T) {
	checker := NewHTTPWithOptions("", 0, TransportOptions{})
	if checker.Transport.ConnectTimeout != 5*time.Second ||
		checker.Transport.TLSHandshakeTimeout != 5*time.Second ||
		checker.Transport.ResponseHeaderTimeout != 5*time.Second ||
		checker.Transport.IdleConnTimeout != 90*time.Second ||
		checker.Transport.MaxIdleConns != 256 ||
		checker.Transport.MaxIdleConnsPerHost != 32 {
		t.Fatalf("unexpected transport defaults %#v", checker.Transport)
	}
}

func TestHTTPCheckerTransportOverrides(t *testing.T) {
	options := TransportOptions{
		ConnectTimeout:        time.Second,
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: 3 * time.Second,
		IdleConnTimeout:       4 * time.Second,
		MaxIdleConns:          12,
		MaxIdleConnsPerHost:   6,
	}
	checker := NewHTTPWithOptions("", 0, options)
	if checker.Transport != options {
		t.Fatalf("expected options to be preserved, got %#v", checker.Transport)
	}
}

func TestHTTPCheckerUsesHTTPProxyAndBodyMatcher(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.String() != "http://target.test/check" {
			t.Fatalf("expected absolute target URL through proxy, got %s", r.URL.String())
		}
		_, _ = w.Write([]byte("proxy-ok"))
	}))
	defer proxyServer.Close()

	result := NewHTTPWithOptions("http://target.test/check", time.Second, TransportOptions{}).Check(context.Background(), model.Proxy{
		Address:  proxyServer.Listener.Addr().String(),
		Protocol: model.ProtocolHTTP,
	})
	if !result.OK {
		t.Fatalf("expected successful check, got %v/%s", result.Error, result.ErrorType)
	}

	checker := NewHTTPWithOptions("http://target.test/check", time.Second, TransportOptions{})
	checker.BodyContains = "missing"
	result = checker.Check(context.Background(), model.Proxy{
		Address:  proxyServer.Listener.Addr().String(),
		Protocol: model.ProtocolHTTP,
	})
	if result.OK || result.ErrorType != errtype.TargetError {
		t.Fatalf("expected body matcher target error, got ok=%t type=%s err=%v", result.OK, result.ErrorType, result.Error)
	}
}

func TestHTTPCheckerFallsBackAcrossTargets(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "bad target", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer proxyServer.Close()

	checker := NewHTTPWithOptions("http://target.test/fail", time.Second, TransportOptions{})
	checker.TargetURLs = []string{"http://target.test/fail", "http://target.test/ok"}
	result := checker.Check(context.Background(), model.Proxy{
		Address:  proxyServer.Listener.Addr().String(),
		Protocol: model.ProtocolHTTP,
	})
	if !result.OK {
		t.Fatalf("expected fallback target success, got %v/%s", result.Error, result.ErrorType)
	}
}

func TestHTTPCheckerReportsHTTPStatus(t *testing.T) {
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad target", http.StatusTooManyRequests)
	}))
	defer proxyServer.Close()

	result := NewHTTPWithOptions("http://target.test/fail", time.Second, TransportOptions{}).Check(context.Background(), model.Proxy{
		Address:  proxyServer.Listener.Addr().String(),
		Protocol: model.ProtocolHTTP,
	})
	if result.OK || result.ErrorType != errtype.HTTPStatus {
		t.Fatalf("expected http_status, got ok=%t type=%s err=%v", result.OK, result.ErrorType, result.Error)
	}
}
