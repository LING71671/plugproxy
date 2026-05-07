package fetcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type testSource struct {
	name    string
	rawURL  string
	delay   time.Duration
	proxies []model.Proxy
	err     error
	started chan struct{}
	release chan struct{}
}

func (s testSource) Name() string {
	return s.name
}

func (s testSource) SourceURL() string {
	return s.rawURL
}

func (s testSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	if s.started != nil {
		close(s.started)
	}
	if s.release != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.release:
		}
	}
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.delay):
		}
	}
	return s.proxies, s.err
}

func TestFetchAllWithWorkersKeepsOrderAndErrors(t *testing.T) {
	wantErr := errors.New("boom")
	results := FetchAllWithWorkers(context.Background(), []source.Source{
		testSource{name: "one", proxies: []model.Proxy{{ID: "http://127.0.0.1:8080"}}},
		testSource{name: "two", err: wantErr},
	}, 2)

	if len(results) != 2 || results[0].Source != "one" || results[1].Source != "two" {
		t.Fatalf("unexpected results %#v", results)
	}
	if len(results[0].Proxies) != 1 || !errors.Is(results[1].Error, wantErr) {
		t.Fatalf("unexpected proxy/error results %#v", results)
	}
	if results[0].Proxies[0].SourceIndex != 0 || results[0].Proxies[0].SourceTotal != 1 {
		t.Fatalf("expected source position annotation, got %#v", results[0].Proxies[0])
	}
}

func TestFetchStreamWithWorkersEmitsCompletedSourceFirst(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	results := FetchStreamWithWorkers(context.Background(), []source.Source{
		testSource{name: "slow", started: started, release: release},
		testSource{name: "fast"},
	}, 2)

	<-started
	first := <-results
	if first.Source != "fast" {
		close(release)
		t.Fatalf("expected fast result first, got %#v", first)
	}
	close(release)
	for range results {
	}
}

func TestFetchStreamWithOptionsLimitsSameHost(t *testing.T) {
	var mu sync.Mutex
	active := 0
	maxActive := 0
	release := make(chan struct{})
	sources := []source.Source{
		concurrentSource{name: "one", rawURL: "https://example.com/one", release: release, active: &active, maxActive: &maxActive, mu: &mu},
		concurrentSource{name: "two", rawURL: "https://example.com/two", release: release, active: &active, maxActive: &maxActive, mu: &mu},
	}

	done := make(chan []Result, 1)
	go func() {
		var collected []Result
		for result := range FetchStreamWithOptions(context.Background(), sources, Options{Workers: 2, PerHostWorkers: 1}) {
			collected = append(collected, result)
		}
		done <- collected
	}()

	time.Sleep(50 * time.Millisecond)
	close(release)
	results := <-done
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %#v", results)
	}
	if maxActive > 1 {
		t.Fatalf("expected per-host concurrency <= 1, got %d", maxActive)
	}
}

func TestFetchStreamWithOptionsAllowsDifferentHosts(t *testing.T) {
	var mu sync.Mutex
	active := 0
	maxActive := 0
	release := make(chan struct{})
	sources := []source.Source{
		concurrentSource{name: "one", rawURL: "https://one.example/one", release: release, active: &active, maxActive: &maxActive, mu: &mu},
		concurrentSource{name: "two", rawURL: "https://two.example/two", release: release, active: &active, maxActive: &maxActive, mu: &mu},
	}

	done := make(chan struct{})
	go func() {
		for range FetchStreamWithOptions(context.Background(), sources, Options{Workers: 2, PerHostWorkers: 1}) {
		}
		close(done)
	}()

	deadline := time.After(time.Second)
	for {
		mu.Lock()
		value := maxActive
		mu.Unlock()
		if value == 2 {
			close(release)
			<-done
			return
		}
		select {
		case <-deadline:
			close(release)
			t.Fatalf("expected different hosts to run concurrently, max=%d", value)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestFetchStreamWithOptionsStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	results := FetchStreamWithOptions(ctx, []source.Source{
		testSource{name: "block", started: started, release: make(chan struct{})},
	}, Options{Workers: 1})

	<-started
	cancel()
	select {
	case <-drainResults(results):
	case <-time.After(time.Second):
		t.Fatal("stream did not close after cancel")
	}
}

func drainResults(results <-chan Result) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for range results {
		}
		close(done)
	}()
	return done
}

type concurrentSource struct {
	name      string
	rawURL    string
	release   <-chan struct{}
	active    *int
	maxActive *int
	mu        *sync.Mutex
}

func (s concurrentSource) Name() string {
	return s.name
}

func (s concurrentSource) SourceURL() string {
	return s.rawURL
}

func (s concurrentSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	s.mu.Lock()
	(*s.active)++
	if *s.active > *s.maxActive {
		*s.maxActive = *s.active
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		(*s.active)--
		s.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.release:
		return nil, nil
	}
}
