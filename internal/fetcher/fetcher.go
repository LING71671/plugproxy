package fetcher

import (
	"context"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/errtype"
	"github.com/LING71671/plugproxy/internal/hostlimit"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Options struct {
	Workers        int
	PerHostWorkers int
}

type Result struct {
	Source    string
	Proxies   []model.Proxy
	Error     error
	ErrorType errtype.Type
	Duration  time.Duration
}

func FetchAll(ctx context.Context, sources []source.Source) []Result {
	return FetchAllWithWorkers(ctx, sources, len(sources))
}

func FetchAllWithWorkers(ctx context.Context, sources []source.Source, workers int) []Result {
	return FetchAllWithOptions(ctx, sources, Options{Workers: workers})
}

func FetchAllWithOptions(ctx context.Context, sources []source.Source, options Options) []Result {
	workers := options.Workers
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
	limiter := hostlimit.New(options.PerHostWorkers)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] = fetchOne(ctx, sources[i], limiter)
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

func FetchStreamWithWorkers(ctx context.Context, sources []source.Source, workers int) <-chan Result {
	return FetchStreamWithOptions(ctx, sources, Options{Workers: workers})
}

func FetchStreamWithOptions(ctx context.Context, sources []source.Source, options Options) <-chan Result {
	results := make(chan Result)
	go func() {
		defer close(results)
		workers := options.Workers
		if workers <= 0 {
			workers = 1
		}
		if workers > len(sources) {
			workers = len(sources)
		}
		if len(sources) == 0 {
			return
		}

		jobs := make(chan int)
		limiter := hostlimit.New(options.PerHostWorkers)
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					result := fetchOne(ctx, sources[i], limiter)
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
			for i := range sources {
				select {
				case <-ctx.Done():
					return
				case jobs <- i:
				}
			}
		}()

		wg.Wait()
	}()
	return results
}

func fetchOne(ctx context.Context, src source.Source, limiter *hostlimit.Limiter) Result {
	start := time.Now()
	release, ok := limiter.Acquire(ctx, sourceURL(src))
	if !ok {
		err := ctx.Err()
		return Result{Source: src.Name(), Error: err, ErrorType: errtype.Classify(err), Duration: time.Since(start)}
	}
	defer release()

	proxies, err := src.Fetch(ctx)
	return Result{
		Source:    src.Name(),
		Proxies:   proxies,
		Error:     err,
		ErrorType: errtype.Classify(err),
		Duration:  time.Since(start),
	}
}

func sourceURL(src source.Source) string {
	if provider, ok := src.(source.URLProvider); ok {
		return provider.SourceURL()
	}
	return ""
}
