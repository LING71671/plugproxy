package scheduler

import (
	"sort"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

type CheckOptions struct {
	Now       time.Time
	MaxChecks int
	CheckTTL  time.Duration
}

type CheckStats struct {
	Total         int
	Selected      int
	SkippedRecent int
	SkippedLimit  int
}

type CheckSchedule struct {
	Selected []model.Proxy
	Stats    CheckStats
}

func ScheduleChecks(proxies []model.Proxy, options CheckOptions) CheckSchedule {
	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}

	stats := CheckStats{Total: len(proxies)}
	candidates := make([]model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if shouldSkipRecent(proxy, now, options.CheckTTL) {
			stats.SkippedRecent++
			continue
		}
		candidates = append(candidates, proxy)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if checkRank(left) != checkRank(right) {
			return checkRank(left) < checkRank(right)
		}
		if !left.LastCheckedAt.Equal(right.LastCheckedAt) {
			return left.LastCheckedAt.Before(right.LastCheckedAt)
		}
		return left.HealthScore > right.HealthScore
	})

	if options.MaxChecks > 0 && options.MaxChecks < len(candidates) {
		stats.SkippedLimit = len(candidates) - options.MaxChecks
		candidates = candidates[:options.MaxChecks]
	}
	stats.Selected = len(candidates)
	return CheckSchedule{Selected: candidates, Stats: stats}
}

func shouldSkipRecent(proxy model.Proxy, now time.Time, ttl time.Duration) bool {
	if ttl <= 0 {
		return false
	}
	if proxy.CheckCount == 0 || proxy.LastCheckedAt.IsZero() {
		return false
	}
	return now.Sub(proxy.LastCheckedAt) < ttl
}

func checkRank(proxy model.Proxy) int {
	switch proxy.Status() {
	case model.HealthUnchecked:
		return 0
	case model.HealthHealthy:
		return 1
	case model.HealthDegraded:
		return 2
	case model.HealthDead:
		return 3
	default:
		return 4
	}
}
