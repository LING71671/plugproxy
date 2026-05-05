package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type GitHubClient struct {
	client   *http.Client
	token    string
	analyzer Analyzer
	workers  int
}

func NewGitHubClient(timeout time.Duration, workers ...int) GitHubClient {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	workerCount := 8
	if len(workers) > 0 && workers[0] > 0 {
		workerCount = workers[0]
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	return GitHubClient{
		client:   &http.Client{Timeout: timeout},
		token:    token,
		analyzer: NewAnalyzer(),
		workers:  workerCount,
	}
}

func (g GitHubClient) DiscoverRepo(ctx context.Context, repo string) DiscoveryReport {
	report := NewReport(repo, "github_repo")
	files := []string{"README.md"}

	for _, dir := range []string{"sources", "proxies", "fetcher", "spider", "crawler", "collector", ".github/workflows"} {
		items, err := g.listContents(ctx, repo, dir)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.Type != "file" || !isInterestingRepoFile(item.Name) {
				continue
			}
			files = append(files, item.Path)
		}
	}

	report.Candidates = append(report.Candidates, g.discoverFiles(ctx, repo, files)...)
	report.Candidates = Deduplicate(report.Candidates)
	if len(report.Candidates) == 0 {
		report.Failures = append(report.Failures, "no candidates found")
	}
	return report
}

func (g GitHubClient) discoverFiles(ctx context.Context, repo string, files []string) []CandidateSource {
	if len(files) == 0 {
		return nil
	}
	workerCount := g.workers
	if workerCount > len(files) {
		workerCount = len(files)
	}

	jobs := make(chan string)
	results := make(chan []CandidateSource)
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for filePath := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results <- g.discoverFile(ctx, repo, filePath, "github:"+repo+":"+filePath)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, filePath := range files {
			select {
			case <-ctx.Done():
				return
			case jobs <- filePath:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	var candidates []CandidateSource
	for result := range results {
		candidates = append(candidates, result...)
	}
	return candidates
}

func (g GitHubClient) SearchRepos(ctx context.Context, query string, limit int) DiscoveryReport {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	report := NewReport(query, "github_search")

	endpoint := "https://api.github.com/search/repositories?q=" + url.QueryEscape(query) + "&sort=updated&per_page=" + fmt.Sprint(limit)
	var response struct {
		Items []struct {
			FullName    string `json:"full_name"`
			Description string `json:"description"`
			HTMLURL     string `json:"html_url"`
		} `json:"items"`
	}
	if err := g.getJSON(ctx, endpoint, &response); err != nil {
		report.Failures = append(report.Failures, err.Error())
		return report
	}

	for _, item := range response.Items {
		candidate := NewCandidate(item.HTMLURL, item.Description, KindCrawlerCodeReference, "github_search:"+query, item.Description)
		candidate.Name = strings.ReplaceAll(item.FullName, "/", "-")
		candidate.AdapterRequired = true
		report.Candidates = append(report.Candidates, candidate)
	}
	report.Candidates = Deduplicate(report.Candidates)
	return report
}

func (g GitHubClient) discoverFile(ctx context.Context, repo, filePath, discoveredFrom string) []CandidateSource {
	content, err := g.fetchRawFile(ctx, repo, filePath)
	if err != nil {
		return nil
	}

	if strings.Contains(filePath, "sources/") || strings.Contains(filePath, "proxies/") {
		return g.analyzer.AnalyzeURLContent(rawGitHubURL(repo, filePath), content, discoveredFrom)
	}
	return g.analyzer.AnalyzeText(content, discoveredFrom)
}

func (g GitHubClient) fetchRawFile(ctx context.Context, repo, filePath string) (string, error) {
	endpoint := "https://api.github.com/repos/" + repo + "/contents/" + strings.TrimPrefix(filePath, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	g.setHeaders(req)
	req.Header.Set("Accept", "application/vnd.github.raw")
	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("github raw %s: %s", filePath, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, defaultSampleLimit))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (g GitHubClient) listContents(ctx context.Context, repo, dir string) ([]githubContent, error) {
	endpoint := "https://api.github.com/repos/" + repo + "/contents/" + strings.TrimPrefix(dir, "/")
	var items []githubContent
	err := g.getJSON(ctx, endpoint, &items)
	return items, err
}

func (g GitHubClient) getJSON(ctx context.Context, endpoint string, value any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	g.setHeaders(req)
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("github api %s: %s", endpoint, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(value)
}

func (g GitHubClient) setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "plugproxy-discover/0.1")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
}

type githubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

func isInterestingRepoFile(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".md", ".txt", ".json", ".yaml", ".yml", ".py", ".go", ".js", ".ts"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func rawGitHubURL(repo, filePath string) string {
	return "https://raw.githubusercontent.com/" + repo + "/HEAD/" + strings.TrimPrefix(filePath, "/")
}
