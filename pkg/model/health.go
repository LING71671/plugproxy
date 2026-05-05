package model

import (
	"math"
	"time"
)

type CheckUpdate struct {
	OK          bool
	Unsupported bool
	Latency     time.Duration
	Error       string
	CheckedAt   time.Time
}

func ApplyCheck(proxy Proxy, update CheckUpdate) Proxy {
	if update.CheckedAt.IsZero() {
		update.CheckedAt = time.Now()
	}
	if proxy.HealthScore == 0 && proxy.CheckCount == 0 {
		proxy.HealthScore = 50
	}

	proxy.CheckCount++
	proxy.LastCheckedAt = update.CheckedAt
	proxy.Latency = update.Latency

	if update.OK {
		proxy.SuccessCount++
		proxy.ConsecutiveFailures = 0
		proxy.LastSuccessAt = update.CheckedAt
		proxy.LastError = ""
		delta := 15
		switch {
		case update.Latency > 0 && update.Latency < time.Second:
			delta += 10
		case update.Latency >= time.Second && update.Latency <= 5*time.Second:
			delta += 3
		}
		proxy.HealthScore = clampScore(proxy.HealthScore + delta)
	} else {
		proxy.FailureCount++
		proxy.ConsecutiveFailures++
		proxy.LastFailureAt = update.CheckedAt
		if update.Unsupported {
			proxy.LastError = "unsupported protocol"
			proxy.HealthScore = clampScore(proxy.HealthScore - 10)
		} else {
			proxy.LastError = update.Error
			proxy.HealthScore = clampScore(proxy.HealthScore - 20*proxy.ConsecutiveFailures)
		}
	}

	proxy.HealthStatus = classify(proxy)
	return proxy
}

func classify(proxy Proxy) HealthStatus {
	if proxy.CheckCount == 0 {
		return HealthUnchecked
	}
	if proxy.HealthScore < 30 || proxy.ConsecutiveFailures >= 3 {
		return HealthDead
	}
	if proxy.HealthScore >= 70 && proxy.SuccessCount > 0 && proxy.ConsecutiveFailures == 0 {
		return HealthHealthy
	}
	return HealthDegraded
}

func clampScore(score int) int {
	return int(math.Max(0, math.Min(100, float64(score))))
}
