package fetcher

import (
	"context"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Result struct {
	Source   string
	Proxies  []model.Proxy
	Error    error
	Duration time.Duration
}

func FetchAll(ctx context.Context, sources []source.Source) []Result {
	return FetchAllWithWorkers(ctx, sources, len(sources))
}

func FetchAllWithWorkers(ctx context.Context, sources []source.Source, workers int) []Result {
	if workers <= 0 {
		workers = 1
	}
	if workers > len(sources) {
		workers = len(sources)
	}
	results := make([]Result, len(sources))
	if len(sources) == 0 {
		return results
	}

	jobs := make(chan int)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				src := sources[i]
				start := time.Now()
				proxies, err := src.Fetch(ctx)
				results[i] = Result{Source: src.Name(), Proxies: proxies, Error: err, Duration: time.Since(start)}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i := range sources {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	wg.Wait()
	return results
}
