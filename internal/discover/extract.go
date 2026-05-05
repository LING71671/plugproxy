package discover

import (
	"encoding/json"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	urlPattern        = regexp.MustCompile(`https?://[A-Za-z0-9\-._~:/?#\[\]@!$&()*+,;=%]+`)
	proxyLinePattern  = regexp.MustCompile(`(?m)^\s*(?:(https?|socks4|socks5)://)?((?:\d{1,3}\.){3}\d{1,3}):(\d{2,5})\s*$`)
	htmlTablePattern  = regexp.MustCompile(`(?is)<table[^>]*>.*?</table>`)
	methodNamePattern = regexp.MustCompile(`(?m)def\s+(freeProxy\d+|[A-Za-z_][A-Za-z0-9_]*Proxy[A-Za-z0-9_]*)\s*\(`)
)

func ExtractURLs(text string) []string {
	matches := urlPattern.FindAllString(text, -1)
	seen := make(map[string]struct{}, len(matches))
	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		normalized := normalizeURL(match)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		urls = append(urls, normalized)
	}
	sort.Strings(urls)
	return urls
}

func normalizeURL(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`\"'()[]{}<>")
	value = strings.Split(value, "](")[0]
	value = strings.Split(value, ",,")[0]
	value = strings.TrimRight(value, ".,;:)]}，。）、")
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func CandidateName(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "unknown-source"
	}
	base := strings.TrimSuffix(path.Base(parsed.Path), path.Ext(parsed.Path))
	if base == "." || base == "/" || base == "" {
		base = strings.Split(parsed.Host, ".")[0]
	}
	base = strings.ToLower(base)
	base = strings.NewReplacer("_", "-", " ", "-").Replace(base)
	if hint := InferProtocolHint(rawURL, ""); hint != "" && !strings.Contains(base, hint) {
		base = base + "-" + hint
	}
	return base
}

func InferProtocolHint(rawURL, content string) string {
	lower := strings.ToLower(rawURL + "\n" + content)
	for _, protocol := range []string{"socks5", "socks4", "https", "http"} {
		if strings.Contains(lower, protocol) {
			return protocol
		}
	}
	return ""
}

func InferFormat(rawURL, content string) SourceFormat {
	lowerURL := strings.ToLower(rawURL)
	trimmed := strings.TrimSpace(content)
	switch {
	case strings.HasSuffix(lowerURL, ".json") || json.Valid([]byte(trimmed)):
		return FormatJSON
	case strings.Contains(strings.ToLower(trimmed), "<html") || htmlTablePattern.MatchString(trimmed):
		return FormatHTML
	case strings.HasSuffix(lowerURL, ".txt") || strings.HasSuffix(lowerURL, ".csv") || proxyLinePattern.MatchString(trimmed):
		return FormatText
	default:
		return FormatUnknown
	}
}

func InferKind(rawURL, content string) SourceKind {
	format := InferFormat(rawURL, content)
	lowerURL := strings.ToLower(rawURL)
	lowerContent := strings.ToLower(content)
	switch {
	case LooksLikeSourceList(content):
		return KindSourceList
	case strings.Contains(lowerURL, "/api/") || strings.Contains(lowerURL, "api.") || strings.Contains(lowerContent, "api"):
		return KindAPI
	case format == FormatJSON:
		return KindJSON
	case format == FormatHTML:
		return KindHTMLTable
	default:
		return KindRawText
	}
}

func LooksLikeProxyList(content string) bool {
	return len(proxyLinePattern.FindAllStringSubmatch(content, 3)) > 0
}

func LooksLikeSourceList(content string) bool {
	lines := strings.Split(content, "\n")
	urlLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			urlLines++
		}
	}
	return urlLines >= 2
}

func LooksLikeCrawlerCode(content string) bool {
	return methodNamePattern.MatchString(content) || strings.Contains(content, "yield") && strings.Contains(content, "proxy")
}

func IsLikelyProxySourceURL(rawURL, evidence string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(parsed.Host)
	path := strings.ToLower(parsed.Path)
	joined := strings.ToLower(host + " " + path + " " + evidence)

	for _, blocked := range []string{
		"127.0.0.1", "localhost", "example.com", "shields.io", "travis-ci.org",
		"readthedocs.org", "readthedocs.io", "hellogithub.com",
	} {
		if strings.Contains(host, blocked) {
			return false
		}
	}
	for _, suffix := range []string{".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		if strings.HasSuffix(path, suffix) {
			return false
		}
	}
	if host == "github.com" {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			return false
		}
	}

	for _, keyword := range []string{
		"proxy", "proxies", "socks", "free-proxy", "freeproxy", "openproxy",
		"daili", "kuaidaili", "kxdaili", "zdaye", "66ip", "89ip", "docip",
		"ip3366", "jiangxianli", "binglx", "ihuan", "proxylist",
	} {
		if strings.Contains(joined, keyword) {
			return true
		}
	}
	return false
}
