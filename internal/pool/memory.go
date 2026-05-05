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

	proxy = normalize(proxy)
	if existing, ok := p.proxies[proxy.ID]; ok {
		proxy = mergeProxy(existing, proxy)
	}
	p.proxies[proxy.ID] = proxy
}

func normalize(proxy model.Proxy) model.Proxy {
	if proxy.ID == "" {
		proxy.ID = string(proxy.Protocol) + "://" + proxy.Address
	}
	if proxy.CreatedAt.IsZero() {
		proxy.CreatedAt = time.Now()
	}
	if proxy.HealthStatus == "" {
		proxy.HealthStatus = model.HealthUnchecked
	}
	if proxy.CheckCount == 0 && proxy.HealthScore == 0 {
		proxy.HealthScore = 50
	}
	return proxy
}

func mergeProxy(existing model.Proxy, incoming model.Proxy) model.Proxy {
	if incoming.CheckCount > 0 || incoming.LastCheckedAt.After(existing.LastCheckedAt) {
		return incoming
	}
	if existing.CheckCount == 0 {
		return incoming
	}

	incoming.Latency = existing.Latency
	incoming.HealthScore = existing.HealthScore
	incoming.HealthStatus = existing.HealthStatus
	incoming.SuccessCount = existing.SuccessCount
	incoming.FailureCount = existing.FailureCount
	incoming.CheckCount = existing.CheckCount
	incoming.ConsecutiveFailures = existing.ConsecutiveFailures
	incoming.LastCheckedAt = existing.LastCheckedAt
	incoming.LastSuccessAt = existing.LastSuccessAt
	incoming.LastFailureAt = existing.LastFailureAt
	incoming.LastError = existing.LastError
	return incoming
}

func (p *MemoryPool) Get(strategy Strategy, filter Filter) (model.Proxy, bool) {
	items := p.List(filter)
	if len(items) == 0 {
		return model.Proxy{}, false
	}

	switch strategy {
	case StrategyFastest:
		sort.SliceStable(items, func(i, j int) bool {
			if healthRank(items[i]) != healthRank(items[j]) {
				return healthRank(items[i]) > healthRank(items[j])
			}
			if items[i].Latency == 0 {
				return false
			}
			if items[j].Latency == 0 {
				return true
			}
			if items[i].Latency != items[j].Latency {
				return items[i].Latency < items[j].Latency
			}
			return items[i].HealthScore > items[j].HealthScore
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			if healthRank(items[i]) != healthRank(items[j]) {
				return healthRank(items[i]) > healthRank(items[j])
			}
			if items[i].HealthScore != items[j].HealthScore {
				return items[i].HealthScore > items[j].HealthScore
			}
			return items[i].CreatedAt.Before(items[j].CreatedAt)
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

func healthRank(proxy model.Proxy) int {
	switch proxy.Status() {
	case model.HealthHealthy:
		return 3
	case model.HealthDegraded:
		return 2
	case model.HealthUnchecked:
		return 1
	default:
		return 0
	}
}
