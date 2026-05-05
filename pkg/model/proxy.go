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

type Proxy struct {
	ID            string        `json:"id"`
	Address       string        `json:"address"`
	Protocol      Protocol      `json:"protocol"`
	Source        string        `json:"source,omitempty"`
	Country       string        `json:"country,omitempty"`
	Latency       time.Duration `json:"latency"`
	SuccessCount  int           `json:"success_count"`
	FailureCount  int           `json:"failure_count"`
	LastCheckedAt time.Time     `json:"last_checked_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
}

func (p Proxy) URL() (*url.URL, error) {
	return url.Parse(string(p.Protocol) + "://" + p.Address)
}

func (p Proxy) Healthy() bool {
	return p.SuccessCount > 0 && p.FailureCount == 0
}
