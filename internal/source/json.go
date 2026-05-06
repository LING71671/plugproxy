package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LING71671/plugproxy/pkg/model"
)

const (
	DefaultJSONTimeout   = DefaultRawTextTimeout
	DefaultJSONBodyLimit = DefaultRawTextBodyLimit
)

type JSONConfig struct {
	ItemsPath     string `json:"items_path,omitempty"`
	ProxyField    string `json:"proxy_field,omitempty"`
	HostField     string `json:"host_field,omitempty"`
	PortField     string `json:"port_field,omitempty"`
	ProtocolField string `json:"protocol_field,omitempty"`
}

type JSONURLSource struct {
	name         string
	rawURL       string
	protocolHint model.Protocol
	headers      map[string]string
	jsonConfig   JSONConfig
	timeout      time.Duration
	bodyLimit    int64
	client       *http.Client
}

type JSONURLOption struct {
	Name         string
	URL          string
	ProtocolHint model.Protocol
	Headers      map[string]string
	JSON         JSONConfig
	Timeout      time.Duration
	BodyLimit    int64
}

func NewJSONURL(option JSONURLOption) JSONURLSource {
	if option.Timeout <= 0 {
		option.Timeout = DefaultJSONTimeout
	}
	if option.BodyLimit <= 0 {
		option.BodyLimit = DefaultJSONBodyLimit
	}
	headers := make(map[string]string, len(option.Headers))
	for key, value := range option.Headers {
		headers[key] = value
	}
	return JSONURLSource{
		name:         option.Name,
		rawURL:       option.URL,
		protocolHint: option.ProtocolHint,
		headers:      headers,
		jsonConfig:   option.JSON,
		timeout:      option.Timeout,
		bodyLimit:    option.BodyLimit,
		client:       &http.Client{Timeout: option.Timeout},
	}
}

func (s JSONURLSource) Name() string {
	if s.name != "" {
		return s.name
	}
	return s.rawURL
}

func (s JSONURLSource) Fetch(ctx context.Context) ([]model.Proxy, error) {
	if s.rawURL == "" {
		return nil, fmt.Errorf("json source %q has empty URL", s.Name())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "plugproxy/0.1")
	for key, value := range s.headers {
		req.Header.Set(key, value)
	}

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
	return ParseJSONProxies(data, s.protocolHint, s.Name(), s.jsonConfig)
}

func ParseJSONProxies(data []byte, protocolHint model.Protocol, sourceName string, config JSONConfig) ([]model.Proxy, error) {
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	items, ok := jsonItems(root, config.ItemsPath)
	if !ok {
		return nil, nil
	}

	now := time.Now()
	seen := make(map[string]struct{})
	proxies := make([]model.Proxy, 0, len(items))
	for _, item := range items {
		proxy, ok := parseJSONItem(item, protocolHint, config)
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
	return proxies, nil
}

func jsonItems(root any, itemsPath string) ([]any, bool) {
	if items, ok := root.([]any); ok {
		return items, true
	}

	object, ok := root.(map[string]any)
	if !ok {
		return nil, false
	}
	if itemsPath != "" {
		if items, ok := object[itemsPath].([]any); ok {
			return items, true
		}
		return nil, false
	}
	for _, key := range []string{"proxies", "data", "items", "results"} {
		if items, ok := object[key].([]any); ok {
			return items, true
		}
	}
	return nil, false
}

func parseJSONItem(item any, protocolHint model.Protocol, config JSONConfig) (model.Proxy, bool) {
	switch value := item.(type) {
	case string:
		return ParseProxyLine(value, protocolHint)
	case map[string]any:
		if proxy, ok := parseJSONProxyField(value, protocolHint, config); ok {
			return proxy, true
		}
		return parseJSONHostPort(value, protocolHint, config)
	default:
		return model.Proxy{}, false
	}
}

func parseJSONProxyField(item map[string]any, protocolHint model.Protocol, config JSONConfig) (model.Proxy, bool) {
	for _, field := range preferredFields(config.ProxyField, []string{"proxy", "url", "address", "addr"}) {
		value, ok := stringField(item, field)
		if !ok {
			continue
		}
		if proxy, ok := ParseProxyLine(value, protocolHint); ok {
			return proxy, true
		}
	}
	return model.Proxy{}, false
}

func parseJSONHostPort(item map[string]any, protocolHint model.Protocol, config JSONConfig) (model.Proxy, bool) {
	host := ""
	for _, field := range preferredFields(config.HostField, []string{"ip", "host"}) {
		if value, ok := stringField(item, field); ok {
			host = value
			break
		}
	}
	if host == "" {
		return model.Proxy{}, false
	}

	port := ""
	for _, field := range preferredFields(config.PortField, []string{"port"}) {
		if value, ok := portField(item, field); ok {
			port = value
			break
		}
	}
	if port == "" {
		return model.Proxy{}, false
	}

	protocol := protocolHint
	for _, field := range preferredFields(config.ProtocolField, []string{"protocol", "type", "scheme"}) {
		if value, ok := stringField(item, field); ok {
			protocol = model.Protocol(strings.ToLower(value))
			break
		}
	}
	if protocol == "" {
		protocol = model.ProtocolHTTP
	}
	if !validProtocol(protocol) {
		return model.Proxy{}, false
	}
	return ParseProxyLine(string(protocol)+"://"+net.JoinHostPort(host, port), protocol)
}

func preferredFields(configured string, defaults []string) []string {
	if configured == "" {
		return defaults
	}
	fields := make([]string, 0, len(defaults)+1)
	fields = append(fields, configured)
	for _, field := range defaults {
		if field != configured {
			fields = append(fields, field)
		}
	}
	return fields
}

func stringField(item map[string]any, key string) (string, bool) {
	value, ok := item[key]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	text = strings.TrimSpace(text)
	return text, text != ""
}

func portField(item map[string]any, key string) (string, bool) {
	value, ok := item[key]
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		typed = strings.TrimSpace(typed)
		if typed == "" {
			return "", false
		}
		return typed, true
	case float64:
		if typed <= 0 || typed != float64(int(typed)) {
			return "", false
		}
		return strconv.Itoa(int(typed)), true
	default:
		return "", false
	}
}
