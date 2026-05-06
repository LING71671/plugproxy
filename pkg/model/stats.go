package model

type ProxyStats struct {
	Total     int                  `json:"total"`
	Healthy   int                  `json:"healthy"`
	Degraded  int                  `json:"degraded"`
	Dead      int                  `json:"dead"`
	Unchecked int                  `json:"unchecked"`
	Protocols map[Protocol]int     `json:"protocols"`
	Statuses  map[HealthStatus]int `json:"statuses"`
	Sources   map[string]int       `json:"sources"`
}

func NewProxyStats(proxies []Proxy) ProxyStats {
	stats := ProxyStats{
		Total:     len(proxies),
		Protocols: make(map[Protocol]int),
		Statuses:  make(map[HealthStatus]int),
		Sources:   make(map[string]int),
	}
	for _, proxy := range proxies {
		stats.Protocols[proxy.Protocol]++
		status := proxy.Status()
		stats.Statuses[status]++
		switch status {
		case HealthHealthy:
			stats.Healthy++
		case HealthDegraded:
			stats.Degraded++
		case HealthDead:
			stats.Dead++
		default:
			stats.Unchecked++
		}
		if proxy.Source != "" {
			stats.Sources[proxy.Source]++
		}
	}
	return stats
}
