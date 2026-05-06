package checker

import (
	"context"
	"testing"
	"time"

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
