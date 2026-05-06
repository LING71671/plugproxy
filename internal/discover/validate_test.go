package discover

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidatorValidatesJSONWithAutoParser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"ip":"1.1.1.1","port":8080,"protocol":"http"}]}`))
	}))
	defer server.Close()

	candidates := []CandidateSource{NewCandidate(server.URL+"/proxies.json", "", KindJSON, "test", "")}
	got := NewValidator(time.Second, 1).Validate(context.Background(), candidates)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %#v", got)
	}
	if got[0].Status != StatusValid || got[0].AdapterRequired || got[0].Recipe == nil || got[0].Recipe.Parser != "json_auto" {
		t.Fatalf("unexpected candidate %#v", got[0])
	}
}

func TestValidatorRejectsJSONWithoutProxies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"name":"empty"}]}`))
	}))
	defer server.Close()

	candidates := []CandidateSource{NewCandidate(server.URL+"/proxies.json", "", KindJSON, "test", "")}
	got := NewValidator(time.Second, 1).Validate(context.Background(), candidates)
	if len(got) != 1 {
		t.Fatalf("expected 1 candidate, got %#v", got)
	}
	if got[0].Status != StatusInvalid {
		t.Fatalf("expected invalid candidate, got %#v", got[0])
	}
}
