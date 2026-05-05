package pool

import (
	"sort"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

type MemoryPool struct {
	mu      sync.RWMutex
	proxies map[string]model.Proxy
}

func NewMemory() *MemoryPool {
	return &MemoryPool{proxies: make(map[string]model.Proxy)}
}

func (p *MemoryPool) Add(proxy model.Proxy) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if proxy.ID == "" {
		proxy.ID = string(proxy.Protocol) + "://" + proxy.Address
	}
	if proxy.CreatedAt.IsZero() {
		proxy.CreatedAt = time.Now()
	}

	p.proxies[proxy.ID] = proxy
}

func (p *MemoryPool) Get(strategy Strategy, filter Filter) (model.Proxy, bool) {
	items := p.List(filter)
	if len(items) == 0 {
		return model.Proxy{}, false
	}

	if strategy == StrategyFastest {
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].Latency < items[j].Latency
		})
	}

	return items[0], true
}

func (p *MemoryPool) List(filter Filter) []model.Proxy {
	p.mu.RLock()
	defer p.mu.RUnlock()

	items := make([]model.Proxy, 0, len(p.proxies))
	for _, proxy := range p.proxies {
		if filter.Protocol != "" && proxy.Protocol != filter.Protocol {
			continue
		}
		if filter.Healthy && !proxy.Healthy() {
			continue
		}
		items = append(items, proxy)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})

	return items
}
