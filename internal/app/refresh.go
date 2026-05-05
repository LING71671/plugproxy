package app

import (
	"context"
	"sync"
	"time"
)

type RefreshOptions struct {
	Fetch FetchOptions
	Check CheckOptions
}

type RefreshStatus struct {
	Status     string      `json:"status"`
	Running    bool        `json:"running"`
	StartedAt  time.Time   `json:"started_at,omitempty"`
	FinishedAt time.Time   `json:"finished_at,omitempty"`
	SkippedAt  time.Time   `json:"skipped_at,omitempty"`
	Fetch      FetchReport `json:"fetch,omitempty"`
	Check      CheckStats  `json:"check,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type refreshState struct {
	mu      sync.Mutex
	running bool
	status  RefreshStatus
}

func (a *App) TriggerRefresh(ctx context.Context, options RefreshOptions) RefreshStatus {
	status, started := a.startRefresh(ctx, options)
	if !started {
		return status
	}
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
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				a.startRefresh(ctx, options)
			}
		}
	}()
}

func (a *App) startRefresh(ctx context.Context, options RefreshOptions) (RefreshStatus, bool) {
	now := time.Now()
	a.refresh.mu.Lock()
	if a.refresh.running {
		status := a.refresh.status
		status.SkippedAt = now
		a.refresh.status = status
		a.refresh.mu.Unlock()
		return status, false
	}
	status := RefreshStatus{Status: "running", Running: true, StartedAt: now}
	a.refresh.running = true
	a.refresh.status = status
	a.refresh.mu.Unlock()

	go a.runRefresh(ctx, options)
	return status, true
}

func (a *App) runRefresh(ctx context.Context, options RefreshOptions) {
	status := RefreshStatus{Status: "running", Running: true, StartedAt: time.Now()}
	fetchReport := a.FetchWithOptions(ctx, options.Fetch)
	status.Fetch = fetchReport
	status.Check = a.CheckWithOptions(ctx, options.Check)
	status.Status = "completed"
	status.Running = false
	status.FinishedAt = time.Now()

	a.refresh.mu.Lock()
	defer a.refresh.mu.Unlock()
	a.refresh.running = false
	a.refresh.status = status
}
