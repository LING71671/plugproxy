package app

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/checker"
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

func New(log *slog.Logger) *App {
	if log == nil {
		log = slog.Default()
	}

	return &App{
		pool: pool.NewMemory(),
		sources: []source.Source{
			source.NewStatic("example", []model.Proxy{
				{Address: "127.0.0.1:8080", Protocol: model.ProtocolHTTP},
			}),
		},
		log: log,
	}
}

func (a *App) Fetch(ctx context.Context) int {
	count := 0
	for _, result := range fetcher.FetchAll(ctx, a.sources) {
		if result.Error != nil {
			a.log.Warn("source fetch failed", "source", result.Source, "error", result.Error)
			continue
		}

		for _, proxy := range result.Proxies {
			a.pool.Add(proxy)
			count++
		}
	}

	return count
}

func (a *App) Check(ctx context.Context, workers int, targetURL string, timeout time.Duration) int {
	if workers <= 0 {
		workers = 32
	}

	items := a.pool.List(pool.Filter{})
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

	healthy := 0
	for result := range results {
		proxy := result.Proxy
		proxy.LastCheckedAt = time.Now()
		proxy.Latency = result.Latency
		if result.OK {
			proxy.SuccessCount++
			healthy++
		} else {
			proxy.FailureCount++
		}
		a.pool.Add(proxy)
	}

	return healthy
}

func (a *App) Pool() pool.Pool {
	return a.pool
}

func (a *App) Serve(addr string) error {
	srv := server.New(a.pool, a.log)
	a.log.Info("api server listening", "addr", addr)
	return http.ListenAndServe(addr, srv.Handler())
}
