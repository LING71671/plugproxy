package app

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/pkg/model"
)

type RefreshOptions struct {
	Fetch  FetchOptions
	Check  CheckOptions
	Policy RefreshPolicy
}

type RefreshPolicy struct {
	Enabled            bool          `json:"enabled"`
	BaseInterval       time.Duration `json:"base_interval"`
	MinInterval        time.Duration `json:"min_interval"`
	MaxInterval        time.Duration `json:"max_interval"`
	Jitter             time.Duration `json:"jitter"`
	MinHealthy         int           `json:"min_healthy"`
	MinHealthyRatio    float64       `json:"min_healthy_ratio"`
	UncheckedThreshold int           `json:"unchecked_threshold"`
	FailureBackoff     float64       `json:"failure_backoff"`
}

type RefreshDecision struct {
	Delay  time.Duration
	NextAt time.Time
	Reason string
}

type RefreshStatus struct {
	Status        string          `json:"status"`
	Running       bool            `json:"running"`
	Phase         string          `json:"phase,omitempty"`
	Progress      RefreshProgress `json:"progress,omitempty"`
	StartedAt     time.Time       `json:"started_at,omitempty"`
	FinishedAt    time.Time       `json:"finished_at,omitempty"`
	SkippedAt     time.Time       `json:"skipped_at,omitempty"`
	SkippedReason string          `json:"skipped_reason,omitempty"`
	Cancelled     bool            `json:"cancelled,omitempty"`
	NextAt        time.Time       `json:"next_at,omitempty"`
	LastReason    string          `json:"last_reason,omitempty"`
	Policy        RefreshPolicy   `json:"policy,omitempty"`
	Fetch         FetchReport     `json:"fetch,omitempty"`
	Check         CheckStats      `json:"check,omitempty"`
	Pipeline      PipelineReport  `json:"pipeline,omitempty"`
	Error         string          `json:"error,omitempty"`
}

type refreshState struct {
	mu      sync.Mutex
	running bool
	status  RefreshStatus
	cancel  context.CancelFunc
}

func (a *App) TriggerRefresh(ctx context.Context, options RefreshOptions) RefreshStatus {
	status, started := a.startRefresh(ctx, options, "manual")
	if !started {
		return status
	}
	return status
}

func (a *App) CancelRefresh() RefreshStatus {
	now := time.Now()
	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	status := a.refresh.status
	if !a.refresh.running || a.refresh.cancel == nil {
		status.Status = "skipped"
		status.Running = false
		status.SkippedAt = now
		status.SkippedReason = "not_running"
		a.refresh.status = status
		return status
	}
	status.Status = "cancelling"
	status.Running = true
	status.Phase = "cancelling"
	status.Cancelled = true
	a.refresh.status = status
	a.refresh.cancel()
	return status
}

func (a *App) RefreshStatus() RefreshStatus {
	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	if a.refresh.status.Status == "" {
		return RefreshStatus{Status: "idle"}
	}
	return a.refresh.status
}

func (a *App) StartAutoRefresh(ctx context.Context, interval time.Duration, options RefreshOptions) {
	if !options.Policy.Enabled && refreshPolicyConfigured(options.Policy) {
		return
	}
	policy := defaultRefreshPolicy(interval, options.Policy)
	if !policy.Enabled {
		return
	}
	options.Policy = policy
	go func() {
		for {
			decision := a.nextRefreshDecision(policy)
			a.setNextRefresh(decision, policy)
			if !sleepContext(ctx, decision.Delay) {
				return
			}
			a.startRefresh(ctx, options, decision.Reason)
			a.setNextRefresh(RefreshDecision{
				Delay:  policy.MinInterval,
				NextAt: time.Now().Add(policy.MinInterval),
				Reason: decision.Reason,
			}, policy)
			if !sleepContext(ctx, policy.MinInterval) {
				return
			}
		}
	}()
}

func (a *App) startRefresh(ctx context.Context, options RefreshOptions, reason string) (RefreshStatus, bool) {
	now := time.Now()
	options.Policy = defaultRefreshPolicy(options.Policy.BaseInterval, options.Policy)
	refreshCtx, cancel := context.WithCancel(ctx)
	a.refresh.mu.Lock()
	if a.refresh.running {
		cancel()
		status := a.refresh.status
		status.Status = "skipped"
		status.Running = false
		status.SkippedAt = now
		status.SkippedReason = "already_running"
		status.LastReason = reason
		status.Policy = options.Policy
		a.refresh.mu.Unlock()
		return status, false
	}
	status := a.refresh.status
	status.Status = "running"
	status.Running = true
	status.Phase = "fetching"
	status.Progress = RefreshProgress{TotalSources: len(a.sources)}
	status.StartedAt = now
	status.FinishedAt = time.Time{}
	status.SkippedAt = time.Time{}
	status.SkippedReason = ""
	status.Cancelled = false
	status.Error = ""
	status.LastReason = reason
	status.Policy = options.Policy
	a.refresh.running = true
	a.refresh.cancel = cancel
	a.refresh.status = status
	a.refresh.mu.Unlock()

	go a.runRefresh(refreshCtx, options)
	return status, true
}

func (a *App) runRefresh(ctx context.Context, options RefreshOptions) {
	startedAt := time.Now()
	options.Policy = defaultRefreshPolicy(options.Policy.BaseInterval, options.Policy)
	options.Fetch.Progress = func(update PipelineProgress) {
		a.updateRefreshProgress(update)
	}
	pipeline := a.FetchCheckWithOptions(ctx, options.Fetch, options.Check)
	a.updateRefreshProgress(PipelineProgress{Phase: "saving", Progress: pipelineProgressFromReport(pipeline)})
	status := RefreshStatus{Status: "completed", Running: false, Phase: "completed", StartedAt: startedAt}
	status.Pipeline = pipeline
	status.Fetch = pipeline.Fetch
	status.Check = pipeline.Check
	status.Policy = options.Policy
	status.Progress = pipelineProgressFromReport(pipeline)
	status.FinishedAt = time.Now()
	if ctx.Err() != nil {
		status.Status = "cancelled"
		status.Phase = "cancelled"
		status.Cancelled = true
		status.Error = ctx.Err().Error()
	}

	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	status.LastReason = a.refresh.status.LastReason
	status.NextAt = a.refresh.status.NextAt
	a.refresh.running = false
	a.refresh.cancel = nil
	a.refresh.status = status
	a.log.Info("refresh completed",
		"status", status.Status,
		"phase", status.Phase,
		"reason", status.LastReason,
		"duration_ms", status.FinishedAt.Sub(status.StartedAt).Milliseconds(),
		"successful_sources", status.Fetch.SuccessfulSources,
		"failed_sources", status.Fetch.FailedSources,
		"skipped_sources", status.Fetch.SkippedSources,
		"scheduled_checks", status.Check.Scheduled,
		"skipped_recent", status.Check.SkippedRecent,
		"skipped_limit", status.Check.SkippedLimit)
}

func (a *App) updateRefreshProgress(update PipelineProgress) {
	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	if !a.refresh.running {
		return
	}
	status := a.refresh.status
	status.Phase = update.Phase
	status.Progress = update.Progress
	a.refresh.status = status
}

func pipelineProgressFromReport(report PipelineReport) RefreshProgress {
	return RefreshProgress{
		TotalSources:      report.Fetch.TotalSources,
		CompletedSources:  report.Fetch.SuccessfulSources + report.Fetch.FailedSources + report.Fetch.SkippedSources,
		SuccessfulSources: report.Fetch.SuccessfulSources,
		FailedSources:     report.Fetch.FailedSources,
		SkippedSources:    report.Fetch.SkippedSources,
		Fetched:           report.Fetch.Fetched,
		Added:             report.Fetch.Added,
		Duplicates:        report.Fetch.Duplicates,
		ScheduledChecks:   report.Check.Scheduled,
		CompletedChecks:   report.Check.Healthy + report.Check.Degraded + report.Check.Dead,
		FailedChecks:      report.Check.Failed,
		UnsupportedChecks: report.Check.Unsupported,
	}
}

func (a *App) nextRefreshDecision(policy RefreshPolicy) RefreshDecision {
	now := time.Now()
	stats := model.NewProxyStats(a.pool.List(pool.Filter{}))
	status := a.RefreshStatus()
	return DecideRefresh(now, stats, status, policy)
}

func DecideRefresh(now time.Time, stats model.ProxyStats, last RefreshStatus, policy RefreshPolicy) RefreshDecision {
	policy = defaultRefreshPolicy(policy.BaseInterval, policy)
	delay := time.Duration(0)
	reason := "idle"

	switch {
	case policy.MinHealthy > 0 && stats.Healthy < policy.MinHealthy:
		reason = "healthy_below_min"
	case policy.MinHealthyRatio > 0 && healthyRatio(stats) < policy.MinHealthyRatio:
		reason = "healthy_ratio_below_min"
	case policy.UncheckedThreshold > 0 && stats.Unchecked >= policy.UncheckedThreshold:
		reason = "unchecked_above_threshold"
	default:
		delay = policy.BaseInterval
		if lastRefreshHadFailure(last) && policy.FailureBackoff > 1 {
			delay = time.Duration(float64(delay) * policy.FailureBackoff)
			reason = "backoff"
		}
		delay = clampDuration(delay, policy.MinInterval, policy.MaxInterval)
		if delay == policy.MaxInterval && reason == "idle" {
			reason = "max_interval"
		}
		delay += jitter(policy.Jitter)
	}

	return RefreshDecision{Delay: delay, NextAt: now.Add(delay), Reason: reason}
}

func defaultRefreshPolicy(base time.Duration, policy RefreshPolicy) RefreshPolicy {
	if base <= 0 {
		base = policy.BaseInterval
	}
	if base <= 0 {
		base = 5 * time.Minute
	}
	if policy.BaseInterval <= 0 {
		policy.BaseInterval = base
	}
	if policy.MinInterval <= 0 {
		policy.MinInterval = 30 * time.Second
	}
	if policy.MaxInterval <= 0 {
		policy.MaxInterval = 30 * time.Minute
	}
	if policy.MaxInterval < policy.MinInterval {
		policy.MaxInterval = policy.MinInterval
	}
	if policy.Jitter < 0 {
		policy.Jitter = 0
	}
	if policy.MinHealthy == 0 {
		policy.MinHealthy = 1
	}
	if policy.UncheckedThreshold == 0 {
		policy.UncheckedThreshold = 100
	}
	if policy.FailureBackoff <= 0 {
		policy.FailureBackoff = 2
	}
	policy.Enabled = true
	return policy
}

func (a *App) setNextRefresh(decision RefreshDecision, policy RefreshPolicy) {
	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	status := a.refresh.status
	if status.Status == "" {
		status.Status = "idle"
	}
	status.NextAt = decision.NextAt
	status.LastReason = decision.Reason
	status.Policy = policy
	a.refresh.status = status
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func healthyRatio(stats model.ProxyStats) float64 {
	if stats.Total == 0 {
		return 0
	}
	return float64(stats.Healthy) / float64(stats.Total)
}

func lastRefreshHadFailure(status RefreshStatus) bool {
	return status.Error != "" ||
		status.Fetch.FailedSources > 0 ||
		status.Fetch.CacheError != ""
}

func clampDuration(value time.Duration, minValue time.Duration, maxValue time.Duration) time.Duration {
	if value < minValue {
		return minValue
	}
	if maxValue > 0 && value > maxValue {
		return maxValue
	}
	return value
}

func jitter(maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(maxJitter) + 1))
}

func refreshPolicyConfigured(policy RefreshPolicy) bool {
	return policy.BaseInterval != 0 ||
		policy.MinInterval != 0 ||
		policy.MaxInterval != 0 ||
		policy.Jitter != 0 ||
		policy.MinHealthy != 0 ||
		policy.MinHealthyRatio != 0 ||
		policy.UncheckedThreshold != 0 ||
		policy.FailureBackoff != 0
}
