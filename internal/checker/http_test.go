package checker

import (
	"context"
	"testing"

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
