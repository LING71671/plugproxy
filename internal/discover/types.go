package discover

import "time"

type SourceFormat string

const (
	FormatUnknown SourceFormat = "unknown"
	FormatText    SourceFormat = "text"
	FormatJSON    SourceFormat = "json"
	FormatHTML    SourceFormat = "html"
)

type SourceKind string

const (
	KindRawText              SourceKind = "raw_text"
	KindJSON                 SourceKind = "json"
	KindHTMLTable            SourceKind = "html_table"
	KindAPI                  SourceKind = "api"
	KindCrawlerCodeReference SourceKind = "crawler_code_reference"
	KindSourceList           SourceKind = "source_list"
)

type CandidateStatus string

const (
	StatusCandidate CandidateStatus = "candidate"
	StatusValid     CandidateStatus = "valid"
	StatusInvalid   CandidateStatus = "invalid"
)

type CandidateSource struct {
	Name            string          `json:"name"`
	URL             string          `json:"url"`
	Format          SourceFormat    `json:"format"`
	ProtocolHint    string          `json:"protocol_hint,omitempty"`
	SourceKind      SourceKind      `json:"source_kind"`
	Confidence      float64         `json:"confidence"`
	Status          CandidateStatus `json:"status"`
	AdapterRequired bool            `json:"adapter_required"`
	DiscoveredFrom  string          `json:"discovered_from"`
	Evidence        string          `json:"evidence,omitempty"`
	Error           string          `json:"error,omitempty"`
	Recipe          *SourceRecipe   `json:"recipe,omitempty"`
}

type SourceRecipe struct {
	Kind         SourceKind   `json:"kind"`
	Format       SourceFormat `json:"format"`
	Parser       string       `json:"parser"`
	URL          string       `json:"url,omitempty"`
	ProtocolHint string       `json:"protocol_hint,omitempty"`
	Notes        string       `json:"notes,omitempty"`
}

type DiscoveryReport struct {
	Query      string            `json:"query,omitempty"`
	Source     string            `json:"source,omitempty"`
	Generated  time.Time         `json:"generated"`
	Candidates []CandidateSource `json:"candidates"`
	Failures   []string          `json:"failures,omitempty"`
}

func NewReport(query, source string) DiscoveryReport {
	return DiscoveryReport{
		Query:     query,
		Source:    source,
		Generated: time.Now(),
	}
}
