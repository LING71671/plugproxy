package pool

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

type MemoryPool struct {
	mu          sync.RWMutex
	proxies     map[string]model.Proxy
	roundRobin  map[string]int
	random      *rand.Rand
	randomMutex sync.Mutex
}

func NewMemory() *MemoryPool {
	return &MemoryPool{
		proxies:    make(map[string]model.Proxy),
		roundRobin: make(map[string]int),
		random:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *MemoryPool) Add(proxy model.Proxy) {
	_ = p.AddAndGet(proxy)
}

func (p *MemoryPool) AddAndGet(proxy model.Proxy) model.Proxy {
	return p.addAndGet(proxy, false)
}

func (p *MemoryPool) AddSeen(proxy model.Proxy) {
	_ = p.AddSeenAndGet(proxy)
}

func (p *MemoryPool) AddSeenAndGet(proxy model.Proxy) model.Proxy {
	return p.addAndGet(proxy, true)
}

func (p *MemoryPool) addAndGet(proxy model.Proxy, seen bool) model.Proxy {
	p.mu.Lock()
	defer p.mu.Unlock()

	proxy = normalize(proxy)
	if existing, ok := p.proxies[proxy.ID]; ok {
		proxy = mergeProxy(existing, proxy, seen)
	} else if seen {
		proxy.LastSeenAt = time.Now()
		if proxy.SeenCount <= 0 {
			proxy.SeenCount = 1
		}
	}
	p.proxies[proxy.ID] = proxy
	return proxy
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

func mergeProxy(existing model.Proxy, incoming model.Proxy, seen bool) model.Proxy {
	if incoming.CheckCount > 0 || incoming.LastCheckedAt.After(existing.LastCheckedAt) {
		incoming.LastSeenAt = existing.LastSeenAt
		incoming.SeenCount = existing.SeenCount
		incoming.LastUsedAt = existing.LastUsedAt
		incoming.UseCount = existing.UseCount
		return incoming
	}
	if existing.CheckCount == 0 {
		if seen {
			incoming.LastSeenAt = time.Now()
			incoming.SeenCount = existing.SeenCount + 1
			if incoming.SeenCount <= 0 {
				incoming.SeenCount = 1
			}
		} else {
			incoming.LastSeenAt = existing.LastSeenAt
			incoming.SeenCount = existing.SeenCount
		}
		incoming.LastUsedAt = existing.LastUsedAt
		incoming.UseCount = existing.UseCount
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
	incoming.LastSeenAt = existing.LastSeenAt
	incoming.SeenCount = existing.SeenCount
	incoming.LastUsedAt = existing.LastUsedAt
	incoming.UseCount = existing.UseCount
	if seen {
		incoming.LastSeenAt = time.Now()
		incoming.SeenCount++
		if incoming.SeenCount <= 0 {
			incoming.SeenCount = 1
		}
	}
	return incoming
}

func (p *MemoryPool) Get(strategy Strategy, filter Filter) (model.Proxy, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	items := p.filteredLocked(filter)
	if len(items) == 0 {
		return model.Proxy{}, false
	}

	p.sortForStrategy(items, strategy)

	index := 0
	switch strategy {
	case StrategyRandom:
		index = p.randomIndex(len(items))
	case StrategyRoundRobin:
		key := filterKey(strategy, filter)
		index = p.roundRobin[key] % len(items)
		p.roundRobin[key] = (index + 1) % len(items)
	}

	selected := items[index]
	selected.LastUsedAt = time.Now()
	selected.UseCount++
	p.proxies[selected.ID] = selected
	return selected, true
}

func (p *MemoryPool) List(filter Filter) []model.Proxy {
	p.mu.RLock()
	defer p.mu.RUnlock()

	items := p.filteredLocked(filter)

	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		}
		return items[i].ID < items[j].ID
	})

	return items
}

func (p *MemoryPool) filteredLocked(filter Filter) []model.Proxy {
	items := make([]model.Proxy, 0, len(p.proxies))
	for _, proxy := range p.proxies {
		if filter.Protocol != "" && proxy.Protocol != filter.Protocol {
			continue
		}
		if filter.Source != "" && proxy.Source != filter.Source {
			continue
		}
		if filter.Status != "" && proxy.Status() != filter.Status {
			continue
		}
		if filter.Healthy && !proxy.Healthy() {
			continue
		}
		if filter.ExcludeDead && proxy.Status() == model.HealthDead {
			continue
		}
		items = append(items, proxy)
	}
	return items
}

func (p *MemoryPool) sortForStrategy(items []model.Proxy, strategy Strategy) {
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
			return tieBreak(items[i], items[j])
		})
	case StrategyLeastRecentlyUsed:
		sort.SliceStable(items, func(i, j int) bool {
			if healthRank(items[i]) != healthRank(items[j]) {
				return healthRank(items[i]) > healthRank(items[j])
			}
			if items[i].LastUsedAt.IsZero() != items[j].LastUsedAt.IsZero() {
				return items[i].LastUsedAt.IsZero()
			}
			if !items[i].LastUsedAt.Equal(items[j].LastUsedAt) {
				return items[i].LastUsedAt.Before(items[j].LastUsedAt)
			}
			if items[i].UseCount != items[j].UseCount {
				return items[i].UseCount < items[j].UseCount
			}
			return tieBreak(items[i], items[j])
		})
	case StrategyWeighted:
		sort.SliceStable(items, func(i, j int) bool {
			if weight(items[i]) != weight(items[j]) {
				return weight(items[i]) > weight(items[j])
			}
			return tieBreak(items[i], items[j])
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			if healthRank(items[i]) != healthRank(items[j]) {
				return healthRank(items[i]) > healthRank(items[j])
			}
			return tieBreak(items[i], items[j])
		})
	}
}

func (p *MemoryPool) randomIndex(length int) int {
	p.randomMutex.Lock()
	defer p.randomMutex.Unlock()
	return p.random.Intn(length)
}

func tieBreak(left model.Proxy, right model.Proxy) bool {
	if left.HealthScore != right.HealthScore {
		return left.HealthScore > right.HealthScore
	}
	if left.SeenCount != right.SeenCount {
		return left.SeenCount > right.SeenCount
	}
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.Before(right.CreatedAt)
	}
	return left.ID < right.ID
}

func weight(proxy model.Proxy) int {
	return healthRank(proxy)*1000 + proxy.HealthScore*10 + proxy.SeenCount*5 - proxy.UseCount*20
}

func filterKey(strategy Strategy, filter Filter) string {
	return fmt.Sprintf("%s|%s|%t|%s|%s|%t", strategy, filter.Protocol, filter.Healthy, filter.Status, filter.Source, filter.ExcludeDead)
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
