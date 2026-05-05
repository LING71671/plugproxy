package source

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

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

func (s RawTextURLSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.rawURL
}

func (s RawTextURLSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	if s.rawURL == "" {
		return nil, fmt.Errorf("raw text source %q has empty URL", s.Name())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "plugproxy/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("source %q returned %s", s.Name(), resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, s.bodyLimit))
	if err != nil {
		return nil, err
	}
	return ParseRawTextProxies(string(data), s.protocolHint, s.Name()), nil
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
