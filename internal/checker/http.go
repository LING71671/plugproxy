package checker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
	xproxy "golang.org/x/net/proxy"
)

type HTTPChecker struct {
	TargetURL string
	Timeout   time.Duration
}

func NewHTTP(targetURL string, timeout time.Duration) HTTPChecker {
	if targetURL == "" {
		targetURL = "https://httpbin.org/ip"
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	return HTTPChecker{TargetURL: targetURL, Timeout: timeout}
}

func (c HTTPChecker) Check(ctx context.Context, proxy model.Proxy) Result {
	switch proxy.Protocol {
	case model.ProtocolHTTP, model.ProtocolHTTPS:
		return c.checkHTTPProxy(ctx, proxy)
	case model.ProtocolSOCKS5:
		return c.checkSOCKS5Proxy(ctx, proxy)
	case model.ProtocolSOCKS4:
		return Result{Proxy: proxy, Unsupported: true, Error: fmt.Errorf("unsupported protocol: %s", proxy.Protocol)}
	default:
		return Result{Proxy: proxy, Unsupported: true, Error: fmt.Errorf("unsupported protocol: %s", proxy.Protocol)}
	}
}

func (c HTTPChecker) checkHTTPProxy(ctx context.Context, proxy model.Proxy) Result {
	proxyURL, err := proxy.URL()
	if err != nil {
		return Result{Proxy: proxy, Error: err}
	}

	client := &http.Client{
		Timeout: c.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.TargetURL, nil)
	if err != nil {
		return Result{Proxy: proxy, Error: err}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Proxy: proxy, Latency: latency, Error: err}
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	return Result{Proxy: proxy, OK: ok, Latency: latency}
}

func (c HTTPChecker) checkSOCKS5Proxy(ctx context.Context, proxy model.Proxy) Result {
	dialer, err := xproxy.SOCKS5("tcp", proxy.Address, nil, xproxy.Direct)
	if err != nil {
		return Result{Proxy: proxy, Error: err}
	}

	client := &http.Client{
		Timeout: c.Timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				return dialWithContext(ctx, dialer, network, address)
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.TargetURL, nil)
	if err != nil {
		return Result{Proxy: proxy, Error: err}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Proxy: proxy, Latency: latency, Error: err}
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 400
	return Result{Proxy: proxy, OK: ok, Latency: latency}
}

func dialWithContext(ctx context.Context, dialer xproxy.Dialer, network, address string) (net.Conn, error) {
	type dialResult struct {
		conn net.Conn
		err  error
	}
	result := make(chan dialResult, 1)
	go func() {
		conn, err := dialer.Dial(network, address)
		result <- dialResult{conn: conn, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case value := <-result:
		return value.conn, value.err
	}
}
