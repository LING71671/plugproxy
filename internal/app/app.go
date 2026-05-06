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
	"github.com/LING71671/plugproxy/internal/scheduler"
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
	MaxChecks  int
	CheckTTL   time.Duration
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
	Total         int `json:"total"`
	Scheduled     int `json:"scheduled"`
	SkippedRecent int `json:"skipped_recent"`
	SkippedLimit  int `json:"skipped_limit"`
	Healthy       int `json:"healthy"`
	Degraded      int `json:"degraded"`
	Dead          int `json:"dead"`
	Unsupported   int `json:"unsupported"`
	Failed        int `json:"failed"`
}

type PipelineReport struct {
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt time.Time   `json:"finished_at,omitempty"`
	Fetch      FetchReport `json:"fetch"`
	Check      CheckStats  `json:"check"`
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

func (a *App) FetchCheckWithOptions(ctx context.Context, fetchOptions FetchOptions, checkOptions CheckOptions) PipelineReport {
	if fetchOptions.Workers <= 0 {
		fetchOptions.Workers = len(a.sources)
	}
	if checkOptions.Workers <= 0 {
		checkOptions.Workers = 32
	}
	cachePath := fetchOptions.CachePath
	if cachePath == "" {
		cachePath = checkOptions.CachePath
	}
	if cachePath == "" {
		cachePath = cache.DefaultPath
	}
	fetchOptions.CachePath = cachePath
	checkOptions.CachePath = cachePath

	report := PipelineReport{
		StartedAt: time.Now(),
		Fetch: FetchReport{
			TotalSources: len(a.sources),
			CachePath:    cachePath,
			Sources:      make([]SourceFetchReport, 0, len(a.sources)),
		},
	}

	seenFetched := make(map[string]struct{})
	seenChecks := make(map[string]struct{})
	scheduleStats := CheckStats{}

	checkJobs := make(chan model.Proxy, max(1, checkOptions.Workers*4))
	checkResults := make(chan checker.Result, max(1, checkOptions.Workers*4))
	var checkWG sync.WaitGroup
	httpChecker := checker.NewHTTP(checkOptions.TargetURL, checkOptions.Timeout)
	for range checkOptions.Workers {
		checkWG.Add(1)
		go func() {
			defer checkWG.Done()
			for proxy := range checkJobs {
				result := httpChecker.Check(ctx, proxy)
				select {
				case <-ctx.Done():
					return
				case checkResults <- result:
				}
			}
		}()
	}

	outcomes := make(chan CheckStats, 1)
	go func() {
		stats := CheckStats{}
		for result := range checkResults {
			checked := applyCheckResult(result)
			if result.Unsupported {
				stats.Unsupported++
			} else if !result.OK {
				stats.Failed++
			}
			switch checked.HealthStatus {
			case model.HealthHealthy:
				stats.Healthy++
			case model.HealthDegraded:
				stats.Degraded++
			case model.HealthDead:
				stats.Dead++
			}
			a.pool.Add(checked)
		}
		outcomes <- stats
	}()

	loadCache := fetchOptions.CacheFallback || fetchOptions.CacheWrite || checkOptions.CacheWrite
	if loadCache {
		proxies, err := cache.Load(cachePath)
		if err == nil {
			report.Fetch.CacheCount = len(proxies)
			for _, proxy := range proxies {
				a.pool.Add(proxy)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			report.Fetch.CacheError = err.Error()
			a.log.Warn("proxy cache preload failed", "path", cachePath, "error", err)
		}
	}

	submitChecks := func(proxies []model.Proxy) bool {
		candidates := make([]model.Proxy, 0, len(proxies))
		for _, proxy := range proxies {
			if _, ok := seenChecks[proxy.ID]; ok {
				continue
			}
			seenChecks[proxy.ID] = struct{}{}
			candidates = append(candidates, proxy)
		}
		if len(candidates) == 0 {
			return true
		}

		maxChecks := 0
		if checkOptions.MaxChecks > 0 {
			remaining := checkOptions.MaxChecks - scheduleStats.Scheduled
			if remaining <= 0 {
				schedule := scheduler.ScheduleChecks(candidates, scheduler.CheckOptions{CheckTTL: checkOptions.CheckTTL})
				scheduleStats.Total += schedule.Stats.Total
				scheduleStats.SkippedRecent += schedule.Stats.SkippedRecent
				scheduleStats.SkippedLimit += schedule.Stats.Selected
				return true
			}
			maxChecks = remaining
		}

		schedule := scheduler.ScheduleChecks(candidates, scheduler.CheckOptions{
			MaxChecks: maxChecks,
			CheckTTL:  checkOptions.CheckTTL,
		})
		scheduleStats.Total += schedule.Stats.Total
		scheduleStats.Scheduled += schedule.Stats.Selected
		scheduleStats.SkippedRecent += schedule.Stats.SkippedRecent
		scheduleStats.SkippedLimit += schedule.Stats.SkippedLimit
		for _, proxy := range schedule.Selected {
			select {
			case <-ctx.Done():
				return false
			case checkJobs <- proxy:
			}
		}
		return true
	}

	if !submitChecks(a.pool.List(checkOptions.Filter)) {
		close(checkJobs)
		checkWG.Wait()
		close(checkResults)
		report.Check = combineCheckStats(scheduleStats, <-outcomes)
		report.FinishedAt = time.Now()
		a.setFetchReport(report.Fetch)
		return report
	}

	for result := range fetcher.FetchStreamWithWorkers(ctx, a.sources, fetchOptions.Workers) {
		sourceReport := SourceFetchReport{
			Name:       result.Source,
			Count:      len(result.Proxies),
			DurationMS: result.Duration.Milliseconds(),
		}
		if result.Error != nil {
			report.Fetch.FailedSources++
			sourceReport.Status = "failed"
			sourceReport.Error = result.Error.Error()
			report.Fetch.Sources = append(report.Fetch.Sources, sourceReport)
			a.log.Warn("source fetch failed", "source", result.Source, "error", result.Error)
			continue
		}

		report.Fetch.SuccessfulSources++
		report.Fetch.Fetched += len(result.Proxies)
		sourceReport.Status = "ok"
		report.Fetch.Sources = append(report.Fetch.Sources, sourceReport)
		checkCandidates := make([]model.Proxy, 0, len(result.Proxies))
		for _, proxy := range result.Proxies {
			id := proxyID(proxy)
			if _, ok := seenFetched[id]; ok {
				report.Fetch.Duplicates++
				continue
			}
			seenFetched[id] = struct{}{}
			merged := a.pool.AddAndGet(proxy)
			report.Fetch.Added++
			if matchesFilter(merged, checkOptions.Filter) {
				checkCandidates = append(checkCandidates, merged)
			}
		}
		if !submitChecks(checkCandidates) {
			break
		}
	}

	if report.Fetch.Added == 0 && fetchOptions.CacheFallback && report.Fetch.CacheCount > 0 {
		report.Fetch.ReusedFromCache = true
		report.Fetch.Added = report.Fetch.CacheCount
	}

	close(checkJobs)
	checkWG.Wait()
	close(checkResults)
	report.Check = combineCheckStats(scheduleStats, <-outcomes)

	if fetchOptions.CacheWrite || checkOptions.CacheWrite {
		if err := cache.Save(cachePath, a.pool.List(pool.Filter{})); err != nil {
			report.Fetch.CacheError = err.Error()
			a.log.Warn("proxy cache write failed", "path", cachePath, "error", err)
		}
	}

	report.FinishedAt = time.Now()
	a.setFetchReport(report.Fetch)
	return report
}

func (a *App) Check(ctx context.Context, workers int, targetURL string, timeout time.Duration) int {
	stats := a.CheckWithFilter(ctx, workers, targetURL, timeout, pool.Filter{})
	return stats.Healthy
}

func (a *App) CheckWithOptions(ctx context.Context, options CheckOptions) CheckStats {
	if options.Workers <= 0 {
		options.Workers = 32
	}

	items := a.pool.List(options.Filter)
	schedule := scheduler.ScheduleChecks(items, scheduler.CheckOptions{
		MaxChecks: options.MaxChecks,
		CheckTTL:  options.CheckTTL,
	})
	stats := a.checkItems(ctx, schedule.Selected, options.Workers, options.TargetURL, options.Timeout)
	stats.Total = schedule.Stats.Total
	stats.Scheduled = schedule.Stats.Selected
	stats.SkippedRecent = schedule.Stats.SkippedRecent
	stats.SkippedLimit = schedule.Stats.SkippedLimit
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

func applyCheckResult(result checker.Result) model.Proxy {
	errText := ""
	if result.Error != nil {
		errText = result.Error.Error()
	}
	return model.ApplyCheck(result.Proxy, model.CheckUpdate{
		OK:          result.OK,
		Unsupported: result.Unsupported,
		Latency:     result.Latency,
		Error:       errText,
		CheckedAt:   time.Now(),
	})
}

func combineCheckStats(schedule CheckStats, outcomes CheckStats) CheckStats {
	return CheckStats{
		Total:         schedule.Total,
		Scheduled:     schedule.Scheduled,
		SkippedRecent: schedule.SkippedRecent,
		SkippedLimit:  schedule.SkippedLimit,
		Healthy:       outcomes.Healthy,
		Degraded:      outcomes.Degraded,
		Dead:          outcomes.Dead,
		Unsupported:   outcomes.Unsupported,
		Failed:        outcomes.Failed,
	}
}

func (a *App) CheckWithFilter(ctx context.Context, workers int, targetURL string, timeout time.Duration, filter pool.Filter) CheckStats {
	return a.CheckWithOptions(ctx, CheckOptions{Workers: workers, TargetURL: targetURL, Timeout: timeout, Filter: filter})
}

func (a *App) checkItems(ctx context.Context, items []model.Proxy, workers int, targetURL string, timeout time.Duration) CheckStats {
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

	stats := CheckStats{}
	for result := range results {
		proxy := applyCheckResult(result)
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

func proxyID(proxy model.Proxy) string {
	if proxy.ID != "" {
		return proxy.ID
	}
	return string(proxy.Protocol) + "://" + proxy.Address
}

func matchesFilter(proxy model.Proxy, filter pool.Filter) bool {
	if filter.Protocol != "" && proxy.Protocol != filter.Protocol {
		return false
	}
	if filter.Source != "" && proxy.Source != filter.Source {
		return false
	}
	if filter.Status != "" && proxy.Status() != filter.Status {
		return false
	}
	if filter.Healthy && !proxy.Healthy() {
		return false
	}
	return true
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
