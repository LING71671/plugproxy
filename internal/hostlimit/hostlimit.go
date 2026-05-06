package hostlimit

import (
	"context"
	"net/url"
	"strings"
	"sync"
)

type Limiter struct {
	perHost int
	mu      sync.Mutex
	hosts   map[string]chan struct{}
}

func New(perHost int) *Limiter {
	return &Limiter{perHost: perHost, hosts: make(map[string]chan struct{})}
}

func (l *Limiter) Acquire(ctx context.Context, rawURL string) (func(), bool) {
	if l == nil || l.perHost <= 0 {
		return func() {}, true
	}
	host := Host(rawURL)
	if host == "" {
		return func() {}, true
	}

	sem := l.hostSemaphore(host)
	select {
	case <-ctx.Done():
		return nil, false
	case sem <- struct{}{}:
		return func() { <-sem }, true
	}
}

func Host(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	return strings.ToLower(host)
}

func (l *Limiter) hostSemaphore(host string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if sem, ok := l.hosts[host]; ok {
		return sem
	}
	sem := make(chan struct{}, l.perHost)
	l.hosts[host] = sem
	return sem
}
