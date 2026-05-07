package scheduler

import (
	"sort"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

type CheckProfile string

const (
	ProfileFull  CheckProfile = "full"
	ProfileSmart CheckProfile = "smart"
)

type CheckOptions struct {
	Now              time.Time
	MaxChecks        int
	CheckTTL         time.Duration
	Profile          CheckProfile
	HealthyCheckTTL  time.Duration
	DegradedCheckTTL time.Duration
	DeadCheckTTL     time.Duration
	DeadBackoffMax   time.Duration
	SkipUnsupported  bool
	ProtocolFair     bool
	SourceFair       bool
	TailBiased       bool
}

type CheckStats struct {
	Total              int
	Selected           int
	SkippedRecent      int
	SkippedLimit       int
	SkippedUnsupported int
	SkippedBackoff     int
	ByProtocol         map[model.Protocol]CheckProtocolStats
	BySource           map[string]CheckSourceStats
}

type CheckProtocolStats struct {
	Total              int `json:"total"`
	Selected           int `json:"selected"`
	SkippedRecent      int `json:"skipped_recent"`
	SkippedLimit       int `json:"skipped_limit"`
	SkippedUnsupported int `json:"skipped_unsupported"`
	SkippedBackoff     int `json:"skipped_backoff"`
}

type CheckSourceStats struct {
	Total              int `json:"total"`
	Selected           int `json:"selected"`
	SkippedRecent      int `json:"skipped_recent"`
	SkippedLimit       int `json:"skipped_limit"`
	SkippedUnsupported int `json:"skipped_unsupported"`
	SkippedBackoff     int `json:"skipped_backoff"`
	SelectedHead       int `json:"selected_head"`
	SelectedMiddle     int `json:"selected_middle"`
	SelectedTail       int `json:"selected_tail"`
	Healthy            int `json:"healthy,omitempty"`
	Degraded           int `json:"degraded,omitempty"`
	Dead               int `json:"dead,omitempty"`
	Unsupported        int `json:"unsupported,omitempty"`
	Failed             int `json:"failed,omitempty"`
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
	options = normalizeOptions(options)

	stats := CheckStats{
		Total:      len(proxies),
		ByProtocol: make(map[model.Protocol]CheckProtocolStats),
		BySource:   make(map[string]CheckSourceStats),
	}
	candidates := make([]model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		addProtocolTotal(&stats, proxy.Protocol)
		addSourceTotal(&stats, proxy.Source)
		if options.SkipUnsupported && proxy.Protocol == model.ProtocolSOCKS4 {
			stats.SkippedUnsupported++
			addProtocolSkippedUnsupported(&stats, proxy.Protocol)
			addSourceSkippedUnsupported(&stats, proxy.Source)
			continue
		}
		skip := skipReason(proxy, now, options)
		switch skip {
		case "recent":
			stats.SkippedRecent++
			addProtocolSkippedRecent(&stats, proxy.Protocol)
			addSourceSkippedRecent(&stats, proxy.Source)
			continue
		case "backoff":
			stats.SkippedBackoff++
			addProtocolSkippedBackoff(&stats, proxy.Protocol)
			addSourceSkippedBackoff(&stats, proxy.Source)
			continue
		}
		candidates = append(candidates, proxy)
	}

	sortCandidates(candidates)

	if options.MaxChecks > 0 && options.MaxChecks < len(candidates) {
		stats.SkippedLimit = len(candidates) - options.MaxChecks
		candidates = selectLimited(candidates, options)
	}
	stats.Selected = len(candidates)
	for _, proxy := range candidates {
		addProtocolSelected(&stats, proxy.Protocol)
		addSourceSelected(&stats, proxy)
	}
	addProtocolSkippedLimit(&stats)
	addSourceSkippedLimit(&stats)
	return CheckSchedule{Selected: candidates, Stats: stats}
}

func normalizeOptions(options CheckOptions) CheckOptions {
	if options.Profile == "" {
		options.Profile = ProfileFull
	}
	if options.Profile == ProfileSmart {
		if options.CheckTTL <= 0 {
			options.CheckTTL = 30 * time.Minute
		}
		if options.HealthyCheckTTL <= 0 {
			options.HealthyCheckTTL = 6 * time.Hour
		}
		if options.DegradedCheckTTL <= 0 {
			options.DegradedCheckTTL = 30 * time.Minute
		}
		if options.DeadCheckTTL <= 0 {
			options.DeadCheckTTL = 12 * time.Hour
		}
		if options.DeadBackoffMax <= 0 {
			options.DeadBackoffMax = 72 * time.Hour
		}
	}
	return options
}

func skipReason(proxy model.Proxy, now time.Time, options CheckOptions) string {
	if proxy.CheckCount == 0 || proxy.LastCheckedAt.IsZero() {
		return ""
	}
	elapsed := now.Sub(proxy.LastCheckedAt)
	ttl := ttlForProxy(proxy, options)
	if ttl <= 0 {
		return ""
	}
	if elapsed >= ttl {
		return ""
	}
	if options.Profile == ProfileSmart && proxy.Status() == model.HealthDead {
		return "backoff"
	}
	return "recent"
}

func ttlForProxy(proxy model.Proxy, options CheckOptions) time.Duration {
	if options.Profile != ProfileSmart {
		return options.CheckTTL
	}
	switch proxy.Status() {
	case model.HealthHealthy:
		return options.HealthyCheckTTL
	case model.HealthDegraded:
		return options.DegradedCheckTTL
	case model.HealthDead:
		return deadBackoffTTL(proxy, options)
	default:
		return options.CheckTTL
	}
}

func deadBackoffTTL(proxy model.Proxy, options CheckOptions) time.Duration {
	ttl := options.DeadCheckTTL
	failures := proxy.ConsecutiveFailures
	for failures > 3 {
		if options.DeadBackoffMax > 0 && ttl >= options.DeadBackoffMax {
			return options.DeadBackoffMax
		}
		ttl *= 2
		failures--
	}
	if options.DeadBackoffMax > 0 && ttl > options.DeadBackoffMax {
		return options.DeadBackoffMax
	}
	return ttl
}

func sortCandidates(candidates []model.Proxy) {
	sort.SliceStable(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		if checkRank(left) != checkRank(right) {
			return checkRank(left) < checkRank(right)
		}
		if !left.LastCheckedAt.Equal(right.LastCheckedAt) {
			return left.LastCheckedAt.Before(right.LastCheckedAt)
		}
		if left.HealthScore != right.HealthScore {
			return left.HealthScore > right.HealthScore
		}
		return left.SeenCount > right.SeenCount
	})
}

func selectLimited(candidates []model.Proxy, options CheckOptions) []model.Proxy {
	maxChecks := options.MaxChecks
	if options.SourceFair {
		return selectSourceFair(candidates, maxChecks, options.TailBiased)
	}
	if !options.ProtocolFair {
		return candidates[:maxChecks]
	}

	buckets := make(map[model.Protocol][]model.Proxy)
	for _, proxy := range candidates {
		buckets[proxy.Protocol] = append(buckets[proxy.Protocol], proxy)
	}
	order := []model.Protocol{model.ProtocolHTTP, model.ProtocolHTTPS, model.ProtocolSOCKS5, model.ProtocolSOCKS4}
	selected := make([]model.Proxy, 0, maxChecks)
	for len(selected) < maxChecks {
		progress := false
		for _, protocol := range order {
			items := buckets[protocol]
			if len(items) == 0 {
				continue
			}
			selected = append(selected, items[0])
			buckets[protocol] = items[1:]
			progress = true
			if len(selected) == maxChecks {
				break
			}
		}
		if !progress {
			break
		}
	}
	return selected
}

func selectSourceFair(candidates []model.Proxy, maxChecks int, tailBiased bool) []model.Proxy {
	buckets := make(map[string][]model.Proxy)
	order := make([]string, 0)
	for _, proxy := range candidates {
		key := sourceKey(proxy.Source)
		if _, ok := buckets[key]; !ok {
			order = append(order, key)
		}
		buckets[key] = append(buckets[key], proxy)
	}
	for key, items := range buckets {
		buckets[key] = orderSourceBucket(items, tailBiased)
	}

	selected := make([]model.Proxy, 0, maxChecks)
	for len(selected) < maxChecks {
		progress := false
		for _, source := range order {
			items := buckets[source]
			if len(items) == 0 {
				continue
			}
			selected = append(selected, items[0])
			buckets[source] = items[1:]
			progress = true
			if len(selected) == maxChecks {
				break
			}
		}
		if !progress {
			break
		}
	}
	return selected
}

func orderSourceBucket(items []model.Proxy, tailBiased bool) []model.Proxy {
	if !tailBiased {
		return items
	}
	ordered := make([]model.Proxy, 0, len(items))
	for rank := 0; rank <= 4; rank++ {
		head := make([]model.Proxy, 0)
		middle := make([]model.Proxy, 0)
		tail := make([]model.Proxy, 0)
		for _, proxy := range items {
			if checkRank(proxy) != rank {
				continue
			}
			switch sourceSegment(proxy) {
			case "tail":
				tail = append(tail, proxy)
			case "middle":
				middle = append(middle, proxy)
			default:
				head = append(head, proxy)
			}
		}
		sortSourceIndexDesc(tail)
		sortSourceIndexDesc(middle)
		sortSourceIndexDesc(head)
		ordered = append(ordered, tail...)
		ordered = append(ordered, middle...)
		ordered = append(ordered, head...)
	}
	return ordered
}

func sortSourceIndexDesc(items []model.Proxy) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].SourceIndex > items[j].SourceIndex
	})
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

func addProtocolTotal(stats *CheckStats, protocol model.Protocol) {
	value := stats.ByProtocol[protocol]
	value.Total++
	stats.ByProtocol[protocol] = value
}

func addProtocolSelected(stats *CheckStats, protocol model.Protocol) {
	value := stats.ByProtocol[protocol]
	value.Selected++
	stats.ByProtocol[protocol] = value
}

func addProtocolSkippedRecent(stats *CheckStats, protocol model.Protocol) {
	value := stats.ByProtocol[protocol]
	value.SkippedRecent++
	stats.ByProtocol[protocol] = value
}

func addProtocolSkippedUnsupported(stats *CheckStats, protocol model.Protocol) {
	value := stats.ByProtocol[protocol]
	value.SkippedUnsupported++
	stats.ByProtocol[protocol] = value
}

func addProtocolSkippedBackoff(stats *CheckStats, protocol model.Protocol) {
	value := stats.ByProtocol[protocol]
	value.SkippedBackoff++
	stats.ByProtocol[protocol] = value
}

func addProtocolSkippedLimit(stats *CheckStats) {
	for protocol, value := range stats.ByProtocol {
		value.SkippedLimit = value.Total - value.Selected - value.SkippedRecent - value.SkippedUnsupported - value.SkippedBackoff
		if value.SkippedLimit < 0 {
			value.SkippedLimit = 0
		}
		stats.ByProtocol[protocol] = value
	}
}

func addSourceTotal(stats *CheckStats, source string) {
	key := sourceKey(source)
	value := stats.BySource[key]
	value.Total++
	stats.BySource[key] = value
}

func addSourceSelected(stats *CheckStats, proxy model.Proxy) {
	key := sourceKey(proxy.Source)
	value := stats.BySource[key]
	value.Selected++
	switch sourceSegment(proxy) {
	case "tail":
		value.SelectedTail++
	case "middle":
		value.SelectedMiddle++
	default:
		value.SelectedHead++
	}
	stats.BySource[key] = value
}

func addSourceSkippedRecent(stats *CheckStats, source string) {
	key := sourceKey(source)
	value := stats.BySource[key]
	value.SkippedRecent++
	stats.BySource[key] = value
}

func addSourceSkippedUnsupported(stats *CheckStats, source string) {
	key := sourceKey(source)
	value := stats.BySource[key]
	value.SkippedUnsupported++
	stats.BySource[key] = value
}

func addSourceSkippedBackoff(stats *CheckStats, source string) {
	key := sourceKey(source)
	value := stats.BySource[key]
	value.SkippedBackoff++
	stats.BySource[key] = value
}

func addSourceSkippedLimit(stats *CheckStats) {
	for source, value := range stats.BySource {
		value.SkippedLimit = value.Total - value.Selected - value.SkippedRecent - value.SkippedUnsupported - value.SkippedBackoff
		if value.SkippedLimit < 0 {
			value.SkippedLimit = 0
		}
		stats.BySource[source] = value
	}
}

func sourceKey(source string) string {
	if source == "" {
		return "unknown"
	}
	return source
}

func sourceSegment(proxy model.Proxy) string {
	if proxy.SourceTotal <= 0 {
		return "head"
	}
	position := float64(proxy.SourceIndex+1) / float64(proxy.SourceTotal)
	switch {
	case position > 0.5:
		return "tail"
	case position > 0.2:
		return "middle"
	default:
		return "head"
	}
}
