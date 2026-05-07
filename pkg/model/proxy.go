package model

import (
	"net/url"
	"time"
)

type Protocol string

const (
	ProtocolHTTP   Protocol = "http"
	ProtocolHTTPS  Protocol = "https"
	ProtocolSOCKS4 Protocol = "socks4"
	ProtocolSOCKS5 Protocol = "socks5"
)

type HealthStatus string

const (
	HealthUnchecked HealthStatus = "unchecked"
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthDead      HealthStatus = "dead"
)

type Proxy struct {
	ID                  string        `json:"id"`
	Address             string        `json:"address"`
	Protocol            Protocol      `json:"protocol"`
	Source              string        `json:"source,omitempty"`
	Country             string        `json:"country,omitempty"`
	Latency             time.Duration `json:"latency"`
	HealthScore         int           `json:"health_score"`
	HealthStatus        HealthStatus  `json:"health_status"`
	SuccessCount        int           `json:"success_count"`
	FailureCount        int           `json:"failure_count"`
	CheckCount          int           `json:"check_count"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	LastCheckedAt       time.Time     `json:"last_checked_at,omitempty"`
	LastSuccessAt       time.Time     `json:"last_success_at,omitempty"`
	LastFailureAt       time.Time     `json:"last_failure_at,omitempty"`
	LastError           string        `json:"last_error,omitempty"`
	LastSeenAt          time.Time     `json:"last_seen_at,omitempty"`
	SeenCount           int           `json:"seen_count,omitempty"`
	LastUsedAt          time.Time     `json:"last_used_at,omitempty"`
	UseCount            int           `json:"use_count,omitempty"`
	SourceIndex         int           `json:"source_index,omitempty"`
	SourceTotal         int           `json:"source_total,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
}

func (p Proxy) URL() (*url.URL, error) {
	return url.Parse(string(p.Protocol) + "://" + p.Address)
}

func (p Proxy) Healthy() bool {
	return p.HealthStatus == HealthHealthy
}

func (p Proxy) Status() HealthStatus {
	if p.HealthStatus != "" {
		return p.HealthStatus
	}
	if p.CheckCount == 0 {
		return HealthUnchecked
	}
	if p.HealthScore >= 70 && p.SuccessCount > 0 && p.ConsecutiveFailures == 0 {
		return HealthHealthy
	}
	if p.HealthScore < 30 || p.ConsecutiveFailures >= 3 {
		return HealthDead
	}
	return HealthDegraded
}
