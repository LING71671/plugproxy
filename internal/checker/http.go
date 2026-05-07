package checker

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LING71671/plugproxy/internal/errtype"
	"github.com/LING71671/plugproxy/pkg/model"
	xproxy "golang.org/x/net/proxy"
)

type HTTPChecker struct {
	TargetURL    string
	TargetURLs   []string
	BodyContains string
	Timeout      time.Duration
	Transport    TransportOptions
}

type TransportOptions struct {
	ConnectTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
}

func NewHTTP(targetURL string, timeout time.Duration) HTTPChecker {
	return NewHTTPWithOptions(targetURL, timeout, TransportOptions{})
}

func NewHTTPWithOptions(targetURL string, timeout time.Duration, transport TransportOptions) HTTPChecker {
	if targetURL == "" {
		targetURL = "https://httpbin.org/ip"
	}
	if timeout <= 0 {
		timeout = 8 * time.Second
	}

	return HTTPChecker{TargetURL: targetURL, TargetURLs: []string{targetURL}, Timeout: timeout, Transport: transport.WithDefaults()}
}

func (o TransportOptions) WithDefaults() TransportOptions {
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 5 * time.Second
	}
	if o.TLSHandshakeTimeout <= 0 {
		o.TLSHandshakeTimeout = 5 * time.Second
	}
	if o.ResponseHeaderTimeout <= 0 {
		o.ResponseHeaderTimeout = 5 * time.Second
	}
	if o.IdleConnTimeout <= 0 {
		o.IdleConnTimeout = 90 * time.Second
	}
	if o.MaxIdleConns <= 0 {
		o.MaxIdleConns = 256
	}
	if o.MaxIdleConnsPerHost <= 0 {
		o.MaxIdleConnsPerHost = 32
	}
	return o
}

func (c HTTPChecker) Check(ctx context.Context, proxy model.Proxy) Result {
	switch proxy.Protocol {
	case model.ProtocolHTTP, model.ProtocolHTTPS:
		return c.checkHTTPProxy(ctx, proxy)
	case model.ProtocolSOCKS5:
		return c.checkSOCKS5Proxy(ctx, proxy)
	case model.ProtocolSOCKS4:
		err := fmt.Errorf("unsupported protocol: %s", proxy.Protocol)
		return Result{Proxy: proxy, Unsupported: true, Error: err, ErrorType: errtype.ProtocolUnsupported}
	default:
		err := fmt.Errorf("unsupported protocol: %s", proxy.Protocol)
		return Result{Proxy: proxy, Unsupported: true, Error: err, ErrorType: errtype.ProtocolUnsupported}
	}
}

func (c HTTPChecker) checkHTTPProxy(ctx context.Context, proxy model.Proxy) Result {
	proxyURL, err := proxy.URL()
	if err != nil {
		return Result{Proxy: proxy, Error: err, ErrorType: errtype.ParseError}
	}

	client := &http.Client{
		Timeout:   c.Timeout,
		Transport: c.newTransport(proxyURL),
	}

	return c.checkTargets(ctx, proxy, client)
}

func (c HTTPChecker) checkSOCKS5Proxy(ctx context.Context, proxy model.Proxy) Result {
	dialer, err := xproxy.SOCKS5("tcp", proxy.Address, nil, netDialer{timeout: c.Transport.ConnectTimeout})
	if err != nil {
		return Result{Proxy: proxy, Error: err, ErrorType: errtype.ConnectionError}
	}

	client := &http.Client{
		Timeout:   c.Timeout,
		Transport: c.newSOCKSTransport(dialer),
	}

	return c.checkTargets(ctx, proxy, client)
}

func (c HTTPChecker) checkTargets(ctx context.Context, proxy model.Proxy, client *http.Client) Result {
	targets := c.targets()
	var last Result
	for _, target := range targets {
		result := c.checkOneTarget(ctx, proxy, client, target)
		if result.OK {
			return result
		}
		last = result
		if ctx.Err() != nil {
			return last
		}
	}
	if last.Error == nil {
		err := fmt.Errorf("no check target configured")
		return Result{Proxy: proxy, Error: err, ErrorType: errtype.ParseError}
	}
	return last
}

func (c HTTPChecker) checkOneTarget(ctx context.Context, proxy model.Proxy, client *http.Client, target string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return Result{Proxy: proxy, Error: err, ErrorType: errtype.ParseError}
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Result{Proxy: proxy, Latency: latency, Error: err, ErrorType: errtype.Classify(err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		err := fmt.Errorf("target %s returned %s", target, resp.Status)
		return Result{Proxy: proxy, OK: false, Latency: latency, Error: err, ErrorType: errtype.HTTPStatus}
	}
	if c.BodyContains != "" {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return Result{Proxy: proxy, OK: false, Latency: latency, Error: err, ErrorType: errtype.TargetError}
		}
		if !strings.Contains(string(body), c.BodyContains) {
			err := fmt.Errorf("target %s body did not contain %q", target, c.BodyContains)
			return Result{Proxy: proxy, OK: false, Latency: latency, Error: err, ErrorType: errtype.TargetError}
		}
	}
	return Result{Proxy: proxy, OK: true, Latency: latency}
}

func (c HTTPChecker) targets() []string {
	targets := make([]string, 0, len(c.TargetURLs)+1)
	seen := make(map[string]struct{})
	for _, target := range c.TargetURLs {
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	if len(targets) == 0 && c.TargetURL != "" {
		targets = append(targets, c.TargetURL)
	}
	if len(targets) == 0 {
		targets = append(targets, "https://httpbin.org/ip")
	}
	return targets
}

func (c HTTPChecker) newTransport(proxyURL *url.URL) *http.Transport {
	options := c.Transport.WithDefaults()
	dialer := &net.Dialer{Timeout: options.ConnectTimeout}
	return &http.Transport{
		Proxy:                 http.ProxyURL(proxyURL),
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   options.TLSHandshakeTimeout,
		ResponseHeaderTimeout: options.ResponseHeaderTimeout,
		IdleConnTimeout:       options.IdleConnTimeout,
		MaxIdleConns:          options.MaxIdleConns,
		MaxIdleConnsPerHost:   options.MaxIdleConnsPerHost,
	}
}

func (c HTTPChecker) newSOCKSTransport(dialer xproxy.Dialer) *http.Transport {
	options := c.Transport.WithDefaults()
	return &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialWithContext(ctx, dialer, network, address)
		},
		TLSHandshakeTimeout:   options.TLSHandshakeTimeout,
		ResponseHeaderTimeout: options.ResponseHeaderTimeout,
		IdleConnTimeout:       options.IdleConnTimeout,
		MaxIdleConns:          options.MaxIdleConns,
		MaxIdleConnsPerHost:   options.MaxIdleConnsPerHost,
	}
}

type netDialer struct {
	timeout time.Duration
}

func (d netDialer) Dial(network string, address string) (net.Conn, error) {
	return net.DialTimeout(network, address, d.timeout)
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
