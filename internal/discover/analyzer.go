package discover

import (
	"sort"
	"strings"
)

type Analyzer struct{}

func NewAnalyzer() Analyzer {
	return Analyzer{}
}

func (a Analyzer) AnalyzeURLContent(rawURL, content, discoveredFrom string) []CandidateSource {
	if LooksLikeSourceList(content) {
		urls := ExtractURLs(content)
		candidates := make([]CandidateSource, 0, len(urls))
		for _, discoveredURL := range urls {
			evidence := evidenceForURL(content, discoveredURL)
			if !IsLikelyProxySourceURL(discoveredURL, evidence) {
				continue
			}
			candidates = append(candidates, NewCandidate(discoveredURL, "", KindSourceList, discoveredFrom, evidence))
		}
		return Deduplicate(candidates)
	}

	if LooksLikeProxyList(content) || InferFormat(rawURL, content) != FormatUnknown {
		candidate := NewCandidate(rawURL, content, InferKind(rawURL, content), discoveredFrom, sampleEvidence(content))
		if candidate.Format == FormatHTML {
			candidate.AdapterRequired = true
		}
		return []CandidateSource{candidate}
	}

	return a.AnalyzeText(content, discoveredFrom)
}

func (a Analyzer) AnalyzeText(content, discoveredFrom string) []CandidateSource {
	urls := ExtractURLs(content)
	candidates := make([]CandidateSource, 0, len(urls))
	codeReference := LooksLikeCrawlerCode(content)
	for _, rawURL := range urls {
		evidence := evidenceForURL(content, rawURL)
		if !IsLikelyProxySourceURL(rawURL, evidence) {
			continue
		}
		kind := InferKind(rawURL, "")
		if codeReference {
			kind = KindCrawlerCodeReference
		}
		candidates = append(candidates, NewCandidate(rawURL, "", kind, discoveredFrom, evidence))
	}
	return Deduplicate(candidates)
}

func NewCandidate(rawURL, content string, kind SourceKind, discoveredFrom, evidence string) CandidateSource {
	format := InferFormat(rawURL, content)
	if format == FormatUnknown {
		switch kind {
		case KindJSON:
			format = FormatJSON
		case KindHTMLTable, KindCrawlerCodeReference:
			format = FormatHTML
		default:
			format = FormatText
		}
	}

	adapterRequired := kind == KindHTMLTable || kind == KindCrawlerCodeReference
	confidence := 0.55
	switch kind {
	case KindSourceList:
		confidence = 0.70
	case KindRawText, KindJSON, KindAPI:
		confidence = 0.75
	case KindHTMLTable:
		confidence = 0.60
	case KindCrawlerCodeReference:
		confidence = 0.65
	}
	if LooksLikeProxyList(content) {
		confidence += 0.15
	}
	if confidence > 0.95 {
		confidence = 0.95
	}

	return CandidateSource{
		Name:            CandidateName(rawURL),
		URL:             rawURL,
		Format:          format,
		ProtocolHint:    InferProtocolHint(rawURL, content),
		SourceKind:      kind,
		Confidence:      confidence,
		Status:          StatusCandidate,
		AdapterRequired: adapterRequired,
		DiscoveredFrom:  discoveredFrom,
		Evidence:        evidence,
		Recipe: &SourceRecipe{
			Kind:         kind,
			Format:       format,
			Parser:       parserFor(kind, format),
			URL:          rawURL,
			ProtocolHint: InferProtocolHint(rawURL, content),
		},
	}
}

func Deduplicate(candidates []CandidateSource) []CandidateSource {
	byURL := make(map[string]CandidateSource, len(candidates))
	for _, candidate := range candidates {
		if candidate.URL == "" {
			continue
		}
		existing, ok := byURL[candidate.URL]
		if !ok || candidate.Confidence > existing.Confidence {
			byURL[candidate.URL] = candidate
		}
	}

	result := make([]CandidateSource, 0, len(byURL))
	for _, candidate := range byURL {
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Confidence == result[j].Confidence {
			return result[i].URL < result[j].URL
		}
		return result[i].Confidence > result[j].Confidence
	})
	return result
}

func parserFor(kind SourceKind, format SourceFormat) string {
	switch {
	case kind == KindSourceList:
		return "source_list_urls"
	case format == FormatJSON:
		return "json_auto"
	case format == FormatHTML:
		return "html_table_auto"
	default:
		return "raw_text_lines"
	}
}

func evidenceForURL(content, rawURL string) string {
	index := strings.Index(content, rawURL)
	if index < 0 {
		return ""
	}
	start := index - 80
	if start < 0 {
		start = 0
	}
	end := index + len(rawURL) + 80
	if end > len(content) {
		end = len(content)
	}
	return strings.TrimSpace(content[start:end])
}

func sampleEvidence(content string) string {
	content = strings.TrimSpace(content)
	if len(content) > 240 {
		return content[:240]
	}
	return content
}
