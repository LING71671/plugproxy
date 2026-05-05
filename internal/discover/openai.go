package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type AIProvider interface {
	Search(ctx context.Context, query string, limit int) (DiscoveryReport, error)
}

func NewAIProvider(name, model, baseURL string, timeout time.Duration) (AIProvider, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "openai":
		return NewOpenAIProvider(model, baseURL, timeout), nil
	case "responses-compatible":
		return NewOpenAIProvider(model, baseURL, timeout), nil
	default:
		return nil, fmt.Errorf("unsupported AI provider %q", name)
	}
}

func NewOpenAIProvider(model, baseURL string, timeout time.Duration) OpenAIProvider {
	if model == "" {
		model = "gpt-5"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return OpenAIProvider{
		apiKey:  firstNonEmpty(os.Getenv("PLUGPROXY_AI_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: timeout},
	}
}

func (p OpenAIProvider) Search(ctx context.Context, query string, limit int) (DiscoveryReport, error) {
	report := NewReport(query, "openai_web_search")
	if p.apiKey == "" {
		return report, errors.New("OPENAI_API_KEY is required for AI web search")
	}
	if limit <= 0 {
		limit = 10
	}

	input := fmt.Sprintf(`你是 plugproxy 的代理源发现助手。请使用 web search 搜索免费代理源、代理池项目、Raw 代理列表和 API 文档。

要求：
- 只输出候选“代理源”，不要输出单个可用代理。
- 候选源必须是公开 URL。
- 标记 format: text/json/html/unknown。
- 标记 source_kind: raw_text/json/html_table/api/crawler_code_reference/source_list。
- AI 结果默认 status=candidate，不要自动启用。
- 最多输出 %d 个候选。

查询：%s

只返回 JSON，格式：
{"candidates":[{"name":"","url":"","format":"text","protocol_hint":"http","source_kind":"raw_text","confidence":0.8,"status":"candidate","adapter_required":false,"discovered_from":"openai_web_search","evidence":""}]}`, limit, query)

	body := map[string]any{
		"model": p.model,
		"tools": []map[string]any{
			{"type": "web_search"},
		},
		"input": input,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return report, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(payload))
	if err != nil {
		return report, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return report, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return report, err
	}
	if resp.StatusCode >= 400 {
		return report, fmt.Errorf("openai responses api: %s: %s", resp.Status, string(data))
	}

	text := extractResponseText(data)
	candidates, err := parseAICandidates(text)
	if err != nil {
		return report, err
	}
	for i := range candidates {
		if candidates[i].Status == "" {
			candidates[i].Status = StatusCandidate
		}
		if candidates[i].DiscoveredFrom == "" {
			candidates[i].DiscoveredFrom = "openai_web_search:" + query
		}
		if candidates[i].Name == "" {
			candidates[i].Name = CandidateName(candidates[i].URL)
		}
	}
	report.Candidates = Deduplicate(candidates)
	return report, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func extractResponseText(data []byte) string {
	var response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return string(data)
	}
	if response.OutputText != "" {
		return response.OutputText
	}
	var builder strings.Builder
	for _, output := range response.Output {
		for _, content := range output.Content {
			if content.Text != "" {
				builder.WriteString(content.Text)
				builder.WriteByte('\n')
			}
		}
	}
	return builder.String()
}

func parseAICandidates(text string) ([]CandidateSource, error) {
	jsonText := extractJSONObject(text)
	if jsonText == "" {
		return nil, errors.New("AI response did not contain JSON")
	}
	var wrapper struct {
		Candidates []CandidateSource `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(jsonText), &wrapper); err != nil {
		var candidates []CandidateSource
		if arrayErr := json.Unmarshal([]byte(jsonText), &candidates); arrayErr == nil {
			return sanitizeAICandidates(candidates), nil
		}
		return nil, err
	}
	return sanitizeAICandidates(wrapper.Candidates), nil
}

func sanitizeAICandidates(candidates []CandidateSource) []CandidateSource {
	result := make([]CandidateSource, 0, len(candidates))
	for _, candidate := range candidates {
		if normalizeURL(candidate.URL) == "" {
			continue
		}
		candidate.URL = normalizeURL(candidate.URL)
		if candidate.Status == "" {
			candidate.Status = StatusCandidate
		}
		if candidate.Format == "" {
			candidate.Format = InferFormat(candidate.URL, "")
		}
		if candidate.SourceKind == "" {
			candidate.SourceKind = InferKind(candidate.URL, "")
		}
		if candidate.Confidence <= 0 || candidate.Confidence > 1 {
			candidate.Confidence = 0.5
		}
		if candidate.AdapterRequired || candidate.SourceKind == KindHTMLTable || candidate.SourceKind == KindCrawlerCodeReference {
			candidate.AdapterRequired = true
		}
		result = append(result, candidate)
	}
	return result
}

func extractJSONObject(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```json")
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}
	startObj := strings.Index(text, "{")
	startArray := strings.Index(text, "[")
	start := startObj
	endChar := byte('}')
	if startArray >= 0 && (startObj < 0 || startArray < startObj) {
		start = startArray
		endChar = ']'
	}
	if start < 0 {
		return ""
	}
	end := strings.LastIndexByte(text, endChar)
	if end <= start {
		return ""
	}
	return text[start : end+1]
}
