package fetcher

import (
	"context"
	"sync"

	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Result struct {
	Source  string
	Proxies []model.Proxy
	Error   error
}

func FetchAll(ctx context.Context, sources []source.Source) []Result {
	results := make([]Result, len(sources))
	var wg sync.WaitGroup

	for i, src := range sources {
		wg.Add(1)
		go func(i int, src source.Source) {
			defer wg.Done()
			proxies, err := src.Fetch(ctx)
			results[i] = Result{Source: src.Name(), Proxies: proxies, Error: err}
		}(i, src)
	}

	wg.Wait()
	return results
}
