package source

import (
	"bufio"
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/LING71671/plugproxy/internal/errtype"
	"github.com/LING71671/plugproxy/pkg/model"
)

const (
	DefaultRawTextTimeout   = 12 * time.Second
	DefaultRawTextBodyLimit = 2 * 1024 * 1024
)

type RawTextURLSource struct {
	name         string
	rawURL       string
	protocolHint model.Protocol
	timeout      time.Duration
	bodyLimit    int64
	client       *http.Client
}

type HTMLTextURLSource struct {
	RawTextURLSource
}

type RawTextURLOption struct {
	Name         string
	URL          string
	ProtocolHint model.Protocol
	Timeout      time.Duration
	BodyLimit    int64
}

func NewRawTextURL(option RawTextURLOption) RawTextURLSource {
	if option.Timeout <= 0 {
		option.Timeout = DefaultRawTextTimeout
	}
	if option.BodyLimit <= 0 {
		option.BodyLimit = DefaultRawTextBodyLimit
	}
	return RawTextURLSource{
		name:         option.Name,
		rawURL:       option.URL,
		protocolHint: option.ProtocolHint,
		timeout:      option.Timeout,
		bodyLimit:    option.BodyLimit,
		client:       &http.Client{Timeout: option.Timeout},
	}
}

func NewHTMLTextURL(option RawTextURLOption) HTMLTextURLSource {
	return HTMLTextURLSource{RawTextURLSource: NewRawTextURL(option)}
}

func (s RawTextURLSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.rawURL
}

func (s RawTextURLSource) SourceURL() string {
	return s.rawURL
}

func (s RawTextURLSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	if s.rawURL == "" {
		return nil, errtype.Wrap(errtype.ParseError, fmt.Errorf("raw text source %q has empty URL", s.Name()))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rawURL, nil)
	if err != nil {
		return nil, errtype.Wrap(errtype.ParseError, err)
	}
	req.Header.Set("User-Agent", "plugproxy/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errtype.Wrap(errtype.HTTPStatus, fmt.Errorf("source %q returned %s", s.Name(), resp.Status))
	}

	data, err := readLimited(resp.Body, s.bodyLimit)
	if err != nil {
		return nil, err
	}
	return ParseRawTextProxies(string(data), s.protocolHint, s.Name()), nil
}

func (s HTMLTextURLSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	if s.rawURL == "" {
		return nil, errtype.Wrap(errtype.ParseError, fmt.Errorf("html text source %q has empty URL", s.Name()))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rawURL, nil)
	if err != nil {
		return nil, errtype.Wrap(errtype.ParseError, err)
	}
	req.Header.Set("User-Agent", "plugproxy/0.1")
	req.Header.Set("Accept", "text/html, text/plain;q=0.9, */*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errtype.Wrap(errtype.HTTPStatus, fmt.Errorf("source %q returned %s", s.Name(), resp.Status))
	}

	data, err := readLimited(resp.Body, s.bodyLimit)
	if err != nil {
		return nil, err
	}
	return ParseHTMLTextProxies(string(data), s.protocolHint, s.Name()), nil
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return io.ReadAll(reader)
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errtype.Wrap(errtype.BodyLimit, fmt.Errorf("source body exceeds limit %d", limit))
	}
	return data, nil
}

func ParseRawTextProxies(content string, protocolHint model.Protocol, sourceName string) []model.Proxy {
	now := time.Now()
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	seen := make(map[string]struct{})
	var proxies []model.Proxy
	for scanner.Scan() {
		proxy, ok := ParseProxyLine(scanner.Text(), protocolHint)
		if !ok {
			continue
		}
		proxy.Source = sourceName
		proxy.CreatedAt = now
		if proxy.ID == "" {
			proxy.ID = string(proxy.Protocol) + "://" + proxy.Address
		}
		if _, exists := seen[proxy.ID]; exists {
			continue
		}
		seen[proxy.ID] = struct{}{}
		proxies = append(proxies, proxy)
	}
	return proxies
}

var htmlProxyPattern = regexp.MustCompile(`(?i)\b(?:(https?|socks4|socks5)://)?((?:\d{1,3}\.){3}\d{1,3}|[a-z0-9][a-z0-9.-]*\.[a-z]{2,})(?::|%3a)(\d{1,5})\b`)
var htmlTableCellPattern = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]+>`)

func ParseHTMLTextProxies(content string, protocolHint model.Protocol, sourceName string) []model.Proxy {
	now := time.Now()
	seen := make(map[string]struct{})
	var proxies []model.Proxy
	for _, match := range htmlProxyPattern.FindAllStringSubmatch(content, -1) {
		protocol := protocolHint
		if match[1] != "" {
			protocol = model.Protocol(strings.ToLower(match[1]))
		}
		addHTMLProxy(&proxies, seen, match[2], match[3], protocol, sourceName, now)
	}

	cells := htmlTableCells(content)
	for index := 0; index+1 < len(cells); index++ {
		host := cells[index]
		port := cells[index+1]
		protocol := protocolHint
		if index+2 < len(cells) {
			if candidate := model.Protocol(strings.ToLower(cells[index+2])); validProtocol(candidate) {
				protocol = candidate
			}
		}
		addHTMLProxy(&proxies, seen, host, port, protocol, sourceName, now)
	}
	return proxies
}

func htmlTableCells(content string) []string {
	matches := htmlTableCellPattern.FindAllStringSubmatch(content, -1)
	cells := make([]string, 0, len(matches))
	for _, match := range matches {
		text := htmlTagPattern.ReplaceAllString(match[1], " ")
		text = html.UnescapeString(text)
		text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
		if text != "" {
			cells = append(cells, text)
		}
	}
	return cells
}

func addHTMLProxy(proxies *[]model.Proxy, seen map[string]struct{}, host, port string, protocol model.Protocol, sourceName string, createdAt time.Time) {
	if !validProtocol(protocol) {
		return
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort <= 0 || parsedPort > 65535 {
		return
	}
	if looksLikeDottedIPv4(host) && net.ParseIP(host) == nil {
		return
	}
	proxy, ok := ParseProxyLine(string(protocol)+"://"+net.JoinHostPort(host, port), protocol)
	if !ok {
		return
	}
	proxy.Source = sourceName
	proxy.CreatedAt = createdAt
	if _, exists := seen[proxy.ID]; exists {
		return
	}
	seen[proxy.ID] = struct{}{}
	*proxies = append(*proxies, proxy)
}

func looksLikeDottedIPv4(host string) bool {
	parts := strings.Split(host, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

func ParseProxyLine(line string, protocolHint model.Protocol) (model.Proxy, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return model.Proxy{}, false
	}
	line = strings.Trim(line, "`\"'")
	if fields := strings.Fields(line); len(fields) > 0 {
		line = fields[0]
	}
	line = strings.TrimRight(line, ",;")

	protocol := protocolHint
	address := line
	if strings.Contains(line, "://") {
		parsed, err := url.Parse(line)
		if err != nil || parsed.Host == "" {
			return model.Proxy{}, false
		}
		protocol = model.Protocol(strings.ToLower(parsed.Scheme))
		address = parsed.Host
	}
	if protocol == "" {
		protocol = model.ProtocolHTTP
	}
	if !validProtocol(protocol) {
		return model.Proxy{}, false
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || port == "" {
		return model.Proxy{}, false
	}
	if net.ParseIP(host) == nil && !looksLikeHostname(host) {
		return model.Proxy{}, false
	}
	return model.Proxy{
		ID:       string(protocol) + "://" + address,
		Address:  address,
		Protocol: protocol,
	}, true
}

func validProtocol(protocol model.Protocol) bool {
	switch protocol {
	case model.ProtocolHTTP, model.ProtocolHTTPS, model.ProtocolSOCKS4, model.ProtocolSOCKS5:
		return true
	default:
		return false
	}
}

func looksLikeHostname(host string) bool {
	return strings.Contains(host, ".") && !strings.ContainsAny(host, " /\\")
}
