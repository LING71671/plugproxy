package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/LING71671/plugproxy/pkg/model"
)

const DefaultBaseURL = "http://127.0.0.1:8899"

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type Options struct {
	BaseURL    string
	HTTPClient *http.Client
}

type GetProxyOptions struct {
	Strategy string
	Protocol model.Protocol
	Healthy  bool
}

type ListOptions struct {
	Protocol model.Protocol
	Healthy  bool
	Status   model.HealthStatus
	Source   string
	Limit    int
	Offset   int
}

type Error struct {
	StatusCode int
	Body       string
}

func (e Error) Error() string {
	return fmt.Sprintf("plugproxy request failed with status %d: %s", e.StatusCode, e.Body)
}

func New(baseURL string) *Client {
	return NewWithOptions(Options{BaseURL: baseURL})
}

func NewWithOptions(options Options) *Client {
	if options.BaseURL == "" {
		options.BaseURL = DefaultBaseURL
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(options.BaseURL, "/"),
		httpClient: options.HTTPClient,
	}
}

func (c *Client) GetProxy(ctx context.Context, options GetProxyOptions) (model.Proxy, error) {
	values := url.Values{}
	if options.Strategy != "" {
		values.Set("strategy", options.Strategy)
	}
	if options.Protocol != "" {
		values.Set("protocol", string(options.Protocol))
	}
	if options.Healthy {
		values.Set("healthy", "true")
	}

	var proxy model.Proxy
	if err := c.doJSON(ctx, http.MethodGet, "/proxy", values, nil, &proxy); err != nil {
		return model.Proxy{}, err
	}
	return proxy, nil
}

func (c *Client) ListProxies(ctx context.Context, options ListOptions) ([]model.Proxy, error) {
	values := listValues(options)
	var proxies []model.Proxy
	if err := c.doJSON(ctx, http.MethodGet, "/proxies", values, nil, &proxies); err != nil {
		return nil, err
	}
	return proxies, nil
}

func (c *Client) Stats(ctx context.Context) (model.ProxyStats, error) {
	var stats model.ProxyStats
	if err := c.doJSON(ctx, http.MethodGet, "/stats", nil, nil, &stats); err != nil {
		return model.ProxyStats{}, err
	}
	return stats, nil
}

func (c *Client) Sources(ctx context.Context) (map[string]any, error) {
	var value map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/sources", nil, nil, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *Client) TriggerRefresh(ctx context.Context) (map[string]any, error) {
	var value map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/refresh", nil, nil, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *Client) RefreshStatus(ctx context.Context) (map[string]any, error) {
	var value map[string]any
	if err := c.doJSON(ctx, http.MethodGet, "/refresh", nil, nil, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *Client) CancelRefresh(ctx context.Context) (map[string]any, error) {
	var value map[string]any
	if err := c.doJSON(ctx, http.MethodPost, "/refresh/cancel", nil, nil, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func listValues(options ListOptions) url.Values {
	values := url.Values{}
	if options.Protocol != "" {
		values.Set("protocol", string(options.Protocol))
	}
	if options.Healthy {
		values.Set("healthy", "true")
	}
	if options.Status != "" {
		values.Set("status", string(options.Status))
	}
	if options.Source != "" {
		values.Set("source", options.Source)
	}
	if options.Limit > 0 {
		values.Set("limit", fmt.Sprint(options.Limit))
	}
	if options.Offset > 0 {
		values.Set("offset", fmt.Sprint(options.Offset))
	}
	return values
}

func (c *Client) doJSON(ctx context.Context, method string, path string, values url.Values, body any, target any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}

	endpoint := c.baseURL + path
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Error{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(data))}
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
