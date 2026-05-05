package discover

import (
	"context"
	"sync"
	"time"
)

type Validator struct {
	http     HTTPClient
	analyzer Analyzer
	workers  int
}

func NewValidator(timeout time.Duration, workers ...int) Validator {
	workerCount := 32
	if len(workers) > 0 && workers[0] > 0 {
		workerCount = workers[0]
	}
	return Validator{
		http:     NewHTTPClient(timeout, defaultSampleLimit),
		analyzer: NewAnalyzer(),
		workers:  workerCount,
	}
}

func (v Validator) Validate(ctx context.Context, candidates []CandidateSource) []CandidateSource {
	if len(candidates) == 0 {
		return nil
	}

	workerCount := v.workers
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}

	jobs := make(chan CandidateSource)
	results := make(chan CandidateSource)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobs {
				results <- v.validateOne(ctx, candidate)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, candidate := range candidates {
			if candidate.URL == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- candidate:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	result := make([]CandidateSource, 0, len(candidates))
	for candidate := range results {
		result = append(result, candidate)
	}
	return Deduplicate(result)
}

func (v Validator) validateOne(ctx context.Context, candidate CandidateSource) CandidateSource {
	content, err := v.http.FetchSample(ctx, candidate.URL)
	if err != nil {
		candidate.Status = StatusInvalid
		candidate.Error = err.Error()
		return candidate
	}

	analyzed := v.analyzer.AnalyzeURLContent(candidate.URL, content, candidate.DiscoveredFrom)
	if len(analyzed) == 0 {
		candidate.Status = StatusInvalid
		candidate.Error = "content does not look like a proxy source"
		return candidate
	}
	candidate.Status = StatusValid
	candidate.Error = ""
	candidate.Evidence = analyzed[0].Evidence
	candidate.Format = analyzed[0].Format
	candidate.SourceKind = analyzed[0].SourceKind
	candidate.ProtocolHint = analyzed[0].ProtocolHint
	if analyzed[0].Confidence > candidate.Confidence {
		candidate.Confidence = analyzed[0].Confidence
	}
	return candidate
}
