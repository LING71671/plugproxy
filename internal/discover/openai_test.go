package discover

import "testing"

func TestParseAICandidatesRejectsInvalidJSON(t *testing.T) {
	if _, err := parseAICandidates("not json"); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestParseAICandidatesSanitizesCandidates(t *testing.T) {
	text := `{
		"candidates": [
			{
				"name": "Example",
				"url": "https://raw.githubusercontent.com/example/proxy/main/http.txt",
				"format": "text",
				"source_kind": "raw_text",
				"confidence": 1.5
			},
			{
				"name": "Bad",
				"url": "not a url"
			}
		]
	}`

	candidates, err := parseAICandidates(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Confidence != 0.5 {
		t.Fatalf("expected confidence fallback, got %v", candidates[0].Confidence)
	}
	if candidates[0].Status != StatusCandidate {
		t.Fatalf("expected candidate status, got %s", candidates[0].Status)
	}
}
