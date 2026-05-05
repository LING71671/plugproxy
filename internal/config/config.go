package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/model"
)

const DefaultPath = "plugproxy.sources.json"

type Config struct {
	Sources []SourceConfig `json:"sources"`
}

type SourceConfig struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	ProtocolHint string `json:"protocol_hint,omitempty"`
	Enabled      *bool  `json:"enabled,omitempty"`
	Timeout      string `json:"timeout,omitempty"`
	BodyLimit    int64  `json:"body_limit,omitempty"`
}

func LoadSources(path string) ([]source.Source, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	return BuildSources(cfg)
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && filepath.Base(path) == DefaultPath {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func BuildSources(cfg Config) ([]source.Source, error) {
	var sources []source.Source
	for _, item := range cfg.Sources {
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		switch item.Type {
		case "", "raw_text_url":
			timeout := source.DefaultRawTextTimeout
			if item.Timeout != "" {
				parsed, err := time.ParseDuration(item.Timeout)
				if err != nil {
					return nil, fmt.Errorf("source %q timeout: %w", item.Name, err)
				}
				timeout = parsed
			}
			sources = append(sources, source.NewRawTextURL(source.RawTextURLOption{
				Name:         item.Name,
				URL:          item.URL,
				ProtocolHint: model.Protocol(item.ProtocolHint),
				Timeout:      timeout,
				BodyLimit:    item.BodyLimit,
			}))
		default:
			return nil, fmt.Errorf("unsupported source type %q for %q", item.Type, item.Name)
		}
	}
	return sources, nil
}

func DefaultConfig() Config {
	enabled := true
	return Config{Sources: []SourceConfig{
		{Name: "proxyscrape-http", Type: "raw_text_url", URL: "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=http&proxy_format=ipport&format=text", ProtocolHint: "http", Enabled: &enabled},
		{Name: "proxyscrape-socks4", Type: "raw_text_url", URL: "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=socks4&proxy_format=ipport&format=text", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "proxyscrape-socks5", Type: "raw_text_url", URL: "https://api.proxyscrape.com/v4/free-proxy-list/get?request=display_proxies&protocol=socks5&proxy_format=ipport&format=text", ProtocolHint: "socks5", Enabled: &enabled},
		{Name: "proxifly-http", Type: "raw_text_url", URL: "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/http/data.txt", ProtocolHint: "http", Enabled: &enabled},
		{Name: "proxifly-https", Type: "raw_text_url", URL: "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/https/data.txt", ProtocolHint: "https", Enabled: &enabled},
		{Name: "proxifly-socks4", Type: "raw_text_url", URL: "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks4/data.txt", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "proxifly-socks5", Type: "raw_text_url", URL: "https://cdn.jsdelivr.net/gh/proxifly/free-proxy-list@main/proxies/protocols/socks5/data.txt", ProtocolHint: "socks5", Enabled: &enabled},
		{Name: "dpangestuw-http", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/http_proxies.txt", ProtocolHint: "http", Enabled: &enabled},
		{Name: "dpangestuw-socks4", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/socks4_proxies.txt", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "dpangestuw-socks5", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/dpangestuw/Free-Proxy/refs/heads/main/socks5_proxies.txt", ProtocolHint: "socks5", Enabled: &enabled},
		{Name: "thespeedx-http", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/http.txt", ProtocolHint: "http", Enabled: &enabled},
		{Name: "thespeedx-socks4", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks4.txt", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "thespeedx-socks5", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/TheSpeedX/SOCKS-List/master/socks5.txt", ProtocolHint: "socks5", Enabled: &enabled},
		{Name: "proxyscraper-http", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/http.txt", ProtocolHint: "http", Enabled: &enabled},
		{Name: "proxyscraper-socks4", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/socks4.txt", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "proxyscraper-socks5", Type: "raw_text_url", URL: "https://raw.githubusercontent.com/ProxyScraper/ProxyScraper/main/socks5.txt", ProtocolHint: "socks5", Enabled: &enabled},
		{Name: "openproxylist-http", Type: "raw_text_url", URL: "https://api.openproxylist.xyz/http.txt", ProtocolHint: "http", Enabled: &enabled},
		{Name: "openproxylist-https", Type: "raw_text_url", URL: "https://api.openproxylist.xyz/https.txt", ProtocolHint: "https", Enabled: &enabled},
		{Name: "openproxylist-socks4", Type: "raw_text_url", URL: "https://api.openproxylist.xyz/socks4.txt", ProtocolHint: "socks4", Enabled: &enabled},
		{Name: "openproxylist-socks5", Type: "raw_text_url", URL: "https://api.openproxylist.xyz/socks5.txt", ProtocolHint: "socks5", Enabled: &enabled},
	}}
}
