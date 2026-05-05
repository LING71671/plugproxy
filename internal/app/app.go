package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/checker"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/fetcher"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/server"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type App struct {
	pool    *pool.MemoryPool
	sources []source.Source
	log     *slog.Logger
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
	count := 0
	seen := make(map[string]struct{})
	for _, result := range fetcher.FetchAllWithWorkers(ctx, a.sources, workers) {
		if result.Error != nil {
			a.log.Warn("source fetch failed", "source", result.Source, "error", result.Error)
			continue
		}

		for _, proxy := range result.Proxies {
			if proxy.ID == "" {
				proxy.ID = string(proxy.Protocol) + "://" + proxy.Address
			}
			if _, ok := seen[proxy.ID]; ok {
				continue
			}
			seen[proxy.ID] = struct{}{}
			a.pool.Add(proxy)
			count++
		}
	}

	return count
}

func (a *App) Check(ctx context.Context, workers int, targetURL string, timeout time.Duration) int {
	stats := a.CheckWithFilter(ctx, workers, targetURL, timeout, pool.Filter{})
	return stats.Healthy
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
	srv := server.New(a.pool, a.log)
	a.log.Info("api server listening", "addr", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
