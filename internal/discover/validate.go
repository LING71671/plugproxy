package discover

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LING71671/plugproxy/internal/hostlimit"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Validator struct {
	http           HTTPClient
	analyzer       Analyzer
	workers        int
	perHostWorkers int
}

type ValidatorOptions struct {
	Timeout        time.Duration
	Workers        int
	PerHostWorkers int
}

func NewValidator(timeout time.Duration, workers ...int) Validator {
	workerCount := 32
	if len(workers) > 0 && workers[0] > 0 {
		workerCount = workers[0]
	}
	return NewValidatorWithOptions(ValidatorOptions{Timeout: timeout, Workers: workerCount})
}

func NewValidatorWithOptions(options ValidatorOptions) Validator {
	workerCount := options.Workers
	if workerCount <= 0 {
		workerCount = 32
	}
	return Validator{
		http:           NewHTTPClient(options.Timeout, defaultSampleLimit),
		analyzer:       NewAnalyzer(),
		workers:        workerCount,
		perHostWorkers: options.PerHostWorkers,
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
	limiter := hostlimit.New(v.perHostWorkers)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range jobs {
				release, ok := limiter.Acquire(ctx, candidate.URL)
				if !ok {
					candidate.Status = StatusInvalid
					if ctx.Err() != nil {
						candidate.Error = ctx.Err().Error()
					}
					results <- candidate
					continue
				}
				validated := v.validateOne(ctx, candidate)
				release()
				results <- validated
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

	if shouldValidateJSON(candidate, content) {
		return v.validateJSON(candidate, content)
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
	candidate.AdapterRequired = analyzed[0].AdapterRequired
	candidate.Recipe = analyzed[0].Recipe
	if analyzed[0].Confidence > candidate.Confidence {
		candidate.Confidence = analyzed[0].Confidence
	}
	return candidate
}

func (v Validator) validateJSON(candidate CandidateSource, content string) CandidateSource {
	protocolHint := candidate.ProtocolHint
	if protocolHint == "" {
		protocolHint = InferProtocolHint(candidate.URL, content)
	}
	proxies, err := source.ParseJSONProxies([]byte(content), model.Protocol(protocolHint), candidate.Name, source.JSONConfig{})
	if err != nil {
		candidate.Status = StatusInvalid
		candidate.Error = err.Error()
		return candidate
	}
	if len(proxies) == 0 {
		candidate.Status = StatusInvalid
		candidate.Error = "json content does not contain parseable proxies"
		return candidate
	}

	kind := candidate.SourceKind
	if kind != KindAPI {
		kind = InferKind(candidate.URL, content)
	}
	if kind != KindAPI {
		kind = KindJSON
	}
	candidate.Status = StatusValid
	candidate.Error = ""
	candidate.Format = FormatJSON
	candidate.SourceKind = kind
	candidate.ProtocolHint = protocolHint
	candidate.AdapterRequired = false
	candidate.Evidence = sampleEvidence(content)
	if candidate.Confidence < 0.9 {
		candidate.Confidence = 0.9
	}
	candidate.Recipe = &SourceRecipe{
		Kind:         kind,
		Format:       FormatJSON,
		Parser:       "json_auto",
		URL:          candidate.URL,
		ProtocolHint: protocolHint,
		Notes:        fmt.Sprintf("parsed %d proxies", len(proxies)),
	}
	return candidate
}

func shouldValidateJSON(candidate CandidateSource, content string) bool {
	return candidate.SourceKind == KindJSON ||
		candidate.SourceKind == KindAPI ||
		candidate.Format == FormatJSON ||
		InferFormat(candidate.URL, content) == FormatJSON
}
