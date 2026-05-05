package discover

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultSampleLimit = 128 * 1024

type HTTPClient struct {
	client *http.Client
	limit  int64
}

func NewHTTPClient(timeout time.Duration, limit int64) HTTPClient {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	if limit <= 0 {
		limit = defaultSampleLimit
	}
	return HTTPClient{
		client: &http.Client{Timeout: timeout},
		limit:  limit,
	}
}

func (c HTTPClient) FetchSample(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "plugproxy-discover/0.1")
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", c.limit-1))

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, c.limit))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
