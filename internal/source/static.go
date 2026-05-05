package source

import (
	"context"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

type StaticSource struct {
	name    string
	proxies []model.Proxy
}

func NewStatic(name string, proxies []model.Proxy) StaticSource {
	now := time.Now()
	for i := range proxies {
		proxies[i].Source = name
		if proxies[i].CreatedAt.IsZero() {
			proxies[i].CreatedAt = now
		}
	}

	return StaticSource{name: name, proxies: proxies}
}

func (s StaticSource) Name() string {
	return s.name
}

func (s StaticSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return append([]model.Proxy(nil), s.proxies...), nil
	}
}
