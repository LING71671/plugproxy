package app

import (
	"runtime"
	"time"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/pkg/model"
)

type MetricsReport struct {
	GeneratedAt time.Time        `json:"generated_at"`
	UptimeMS    int64            `json:"uptime_ms"`
	Pool        model.ProxyStats `json:"pool"`
	Fetch       FetchReport      `json:"fetch"`
	Check       CheckStats       `json:"check"`
	Refresh     RefreshStatus    `json:"refresh"`
	Runtime     RuntimeMetrics   `json:"runtime"`
	Config      MetricsConfig    `json:"config"`
}

type RuntimeMetrics struct {
	Goroutines int    `json:"goroutines"`
	Alloc      uint64 `json:"alloc"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"num_gc"`
}

type MetricsConfig struct {
	SourceWorkers   int           `json:"source_workers"`
	PerHostWorkers  int           `json:"per_host_workers"`
	CheckWorkers    int           `json:"check_workers"`
	MaxChecks       int           `json:"max_checks"`
	CheckProfile    string        `json:"check_profile"`
	SkipUnsupported bool          `json:"skip_unsupported"`
	ProtocolFair    bool          `json:"protocol_fair"`
	SourceFair      bool          `json:"source_fair"`
	TailBiased      bool          `json:"tail_biased"`
	CachePath       string        `json:"cache_path"`
	RefreshPolicy   RefreshPolicy `json:"refresh_policy"`
}

func (a *App) Metrics(options RefreshOptions) MetricsReport {
	now := time.Now()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	cachePath := options.Fetch.CachePath
	if cachePath == "" {
		cachePath = options.Check.CachePath
	}
	if cachePath == "" {
		cachePath = cache.DefaultPath
	}
	return MetricsReport{
		GeneratedAt: now,
		UptimeMS:    now.Sub(a.startedAt).Milliseconds(),
		Pool:        model.NewProxyStats(a.pool.List(pool.Filter{})),
		Fetch:       a.LastFetchReport(),
		Check:       a.LastCheckStats(),
		Refresh:     a.RefreshStatus(),
		Runtime: RuntimeMetrics{
			Goroutines: runtime.NumGoroutine(),
			Alloc:      mem.Alloc,
			HeapAlloc:  mem.HeapAlloc,
			Sys:        mem.Sys,
			NumGC:      mem.NumGC,
		},
		Config: MetricsConfig{
			SourceWorkers:   options.Fetch.Workers,
			PerHostWorkers:  options.Fetch.PerHostWorkers,
			CheckWorkers:    options.Check.Workers,
			MaxChecks:       options.Check.MaxChecks,
			CheckProfile:    string(options.Check.Profile),
			SkipUnsupported: options.Check.SkipUnsupported,
			ProtocolFair:    options.Check.ProtocolFair,
			SourceFair:      options.Check.SourceFair,
			TailBiased:      options.Check.TailBiased,
			CachePath:       cachePath,
			RefreshPolicy:   options.Policy,
		},
	}
}
