package checker

import (
	"context"
	"net/http"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
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
