package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/checker"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/fetcher"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/server"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type App struct {
	pool            *pool.MemoryPool
	sources         []source.Source
	log             *slog.Logger
	reportMu        sync.RWMutex
	lastFetchReport FetchReport
	refresh         refreshState
}

type FetchOptions struct {
	Workers       int
	CachePath     string
	CacheFallback bool
	CacheWrite    bool
}

type CheckOptions struct {
	Workers    int
	TargetURL  string
	Timeout    time.Duration
	Filter     pool.Filter
	CachePath  string
	CacheWrite bool
}

type FetchReport struct {
	TotalSources      int                 `json:"total_sources"`
	SuccessfulSources int                 `json:"successful_sources"`
	FailedSources     int                 `json:"failed_sources"`
	Fetched           int                 `json:"fetched"`
	Added             int                 `json:"added"`
	Duplicates        int                 `json:"duplicates"`
	ReusedFromCache   bool                `json:"reused_from_cache"`
	CachePath         string              `json:"cache_path,omitempty"`
	CacheCount        int                 `json:"cache_count,omitempty"`
	CacheError        string              `json:"cache_error,omitempty"`
	Sources           []SourceFetchReport `json:"sources"`
}

type SourceFetchReport struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Count      int    `json:"count"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type CheckStats struct {
	Total       int `json:"total"`
	Healthy     int `json:"healthy"`
	Degraded    int `json:"degraded"`
	Dead        int `json:"dead"`
	Unsupported int `json:"unsupported"`
	Failed      int `json:"failed"`
}

func New(log *slog.Logger) *App {
	sources, err := config.LoadSources(config.DefaultPath)
	if err != nil {
		if log == nil {
			log = slog.Default()
		}
		log.Warn("load default sources failed", "error", err)
	}
	return NewWithSources(log, sources)
}

func NewWithSources(log *slog.Logger, sources []source.Source) *App {
	if log == nil {
		log = slog.Default()
	}

	return &App{
		pool:    pool.NewMemory(),
		sources: sources,
		log:     log,
	}
}

func (a *App) Fetch(ctx context.Context) int {
	return a.FetchWithWorkers(ctx, len(a.sources))
}

func (a *App) FetchWithWorkers(ctx context.Context, workers int) int {
	return a.FetchWithOptions(ctx, FetchOptions{
		Workers:       workers,
		CachePath:     cache.DefaultPath,
		CacheFallback: true,
		CacheWrite:    true,
	}).Added
}

func (a *App) FetchWithOptions(ctx context.Context, options FetchOptions) FetchReport {
	if options.Workers <= 0 {
		options.Workers = len(a.sources)
	}
	if options.CachePath == "" {
		options.CachePath = cache.DefaultPath
	}

	count := 0
	report := FetchReport{
		TotalSources: len(a.sources),
		CachePath:    options.CachePath,
		Sources:      make([]SourceFetchReport, 0, len(a.sources)),
	}
	if options.CacheFallback || options.CacheWrite {
		proxies, err := cache.Load(options.CachePath)
		if err == nil {
			report.CacheCount = len(proxies)
			for _, proxy := range proxies {
				a.pool.Add(proxy)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			report.CacheError = err.Error()
			a.log.Warn("proxy cache preload failed", "path", options.CachePath, "error", err)
		}
	}

	seen := make(map[string]struct{})
	for _, result := range fetcher.FetchAllWithWorkers(ctx, a.sources, options.Workers) {
		sourceReport := SourceFetchReport{
			Name:       result.Source,
			Count:      len(result.Proxies),
			DurationMS: result.Duration.Milliseconds(),
		}
		if result.Error != nil {
			report.FailedSources++
			sourceReport.Status = "failed"
			sourceReport.Error = result.Error.Error()
			report.Sources = append(report.Sources, sourceReport)
			a.log.Warn("source fetch failed", "source", result.Source, "error", result.Error)
			continue
		}

		report.SuccessfulSources++
		report.Fetched += len(result.Proxies)
		sourceReport.Status = "ok"
		report.Sources = append(report.Sources, sourceReport)
		for _, proxy := range result.Proxies {
			if proxy.ID == "" {
				proxy.ID = string(proxy.Protocol) + "://" + proxy.Address
			}
			if _, ok := seen[proxy.ID]; ok {
				report.Duplicates++
				continue
			}
			seen[proxy.ID] = struct{}{}
			a.pool.Add(proxy)
			count++
		}
	}

	report.Added = count
	if report.Added > 0 && options.CacheWrite {
		if err := cache.Save(options.CachePath, a.pool.List(pool.Filter{})); err != nil {
			report.CacheError = err.Error()
			a.log.Warn("proxy cache write failed", "path", options.CachePath, "error", err)
		}
	}
	if report.Added == 0 && options.CacheFallback {
		proxies, err := cache.Load(options.CachePath)
		if err != nil {
			report.CacheError = err.Error()
			a.log.Warn("proxy cache fallback failed", "path", options.CachePath, "error", err)
		} else {
			report.ReusedFromCache = true
			report.CacheCount = len(proxies)
			for _, proxy := range proxies {
				a.pool.Add(proxy)
			}
			report.Added = len(proxies)
		}
	}

	a.setFetchReport(report)
	return report
}

func (a *App) Check(ctx context.Context, workers int, targetURL string, timeout time.Duration) int {
	stats := a.CheckWithFilter(ctx, workers, targetURL, timeout, pool.Filter{})
	return stats.Healthy
}

func (a *App) CheckWithOptions(ctx context.Context, options CheckOptions) CheckStats {
	stats := a.CheckWithFilter(ctx, options.Workers, options.TargetURL, options.Timeout, options.Filter)
	if options.CacheWrite {
		if options.CachePath == "" {
			options.CachePath = cache.DefaultPath
		}
		if err := cache.Save(options.CachePath, a.pool.List(pool.Filter{})); err != nil {
			a.log.Warn("proxy cache write failed", "path", options.CachePath, "error", err)
		}
	}
	return stats
}

func (a *App) CheckWithFilter(ctx context.Context, workers int, targetURL string, timeout time.Duration, filter pool.Filter) CheckStats {
	if workers <= 0 {
		workers = 32
	}

	items := a.pool.List(filter)
	jobs := make(chan model.Proxy)
	results := make(chan checker.Result)
	httpChecker := checker.NewHTTP(targetURL, timeout)

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for proxy := range jobs {
				result := httpChecker.Check(ctx, proxy)
				select {
				case <-ctx.Done():
					return
				case results <- result:
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, proxy := range items {
			select {
			case <-ctx.Done():
				return
			case jobs <- proxy:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	stats := CheckStats{Total: len(items)}
	for result := range results {
		errText := ""
		if result.Error != nil {
			errText = result.Error.Error()
		}
		proxy := model.ApplyCheck(result.Proxy, model.CheckUpdate{
			OK:          result.OK,
			Unsupported: result.Unsupported,
			Latency:     result.Latency,
			Error:       errText,
			CheckedAt:   time.Now(),
		})
		if result.Unsupported {
			stats.Unsupported++
		} else if !result.OK {
			stats.Failed++
		}
		switch proxy.HealthStatus {
		case model.HealthHealthy:
			stats.Healthy++
		case model.HealthDegraded:
			stats.Degraded++
		case model.HealthDead:
			stats.Dead++
		}
		a.pool.Add(proxy)
	}

	return stats
}

func (a *App) Pool() pool.Pool {
	return a.pool
}

func (a *App) Serve(addr string) error {
	return a.ServeWithRefresh(addr, RefreshOptions{})
}

func (a *App) ServeWithRefresh(addr string, refreshOptions RefreshOptions) error {
	a.log.Info("api server listening", "addr", addr)
	return http.ListenAndServe(addr, a.Handler(refreshOptions))
}

func (a *App) Handler(refreshOptions RefreshOptions) http.Handler {
	return server.New(a.pool, a.log).WithSourceReport(func() any {
		return a.LastFetchReport()
	}).WithRefresh(func(ctx context.Context) any {
		return a.TriggerRefresh(ctx, refreshOptions)
	}, func() any {
		return a.RefreshStatus()
	}).Handler()
}

func (a *App) LastFetchReport() FetchReport {
	a.reportMu.RLock()
	defer a.reportMu.RUnlock()
	return a.lastFetchReport
}

func (a *App) setFetchReport(report FetchReport) {
	a.reportMu.Lock()
	defer a.reportMu.Unlock()
	a.lastFetchReport = report
}
