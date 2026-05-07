package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const DefaultAppPath = "plugproxy.config.json"

type AppConfig struct {
	Version     int             `json:"version"`
	SourcesPath string          `json:"sources_path,omitempty"`
	Server      ServerConfig    `json:"server,omitempty"`
	Cache       CacheConfig     `json:"cache,omitempty"`
	Fetch       FetchConfig     `json:"fetch,omitempty"`
	Check       CheckConfig     `json:"check,omitempty"`
	Scheduler   SchedulerConfig `json:"scheduler,omitempty"`
	Refresh     RefreshConfig   `json:"refresh,omitempty"`
	Logging     LoggingConfig   `json:"logging,omitempty"`
}

type ServerConfig struct {
	Addr            string `json:"addr,omitempty"`
	ShutdownTimeout string `json:"shutdown_timeout,omitempty"`
}

type CacheConfig struct {
	Path           string `json:"path,omitempty"`
	Fallback       *bool  `json:"fallback,omitempty"`
	MaxEntries     int    `json:"max_entries,omitempty"`
	DropDeadAfter  string `json:"drop_dead_after,omitempty"`
	DropStaleAfter string `json:"drop_stale_after,omitempty"`
}

type FetchConfig struct {
	SourceWorkers          int    `json:"source_workers,omitempty"`
	PerHostWorkers         int    `json:"per_host_workers,omitempty"`
	SourceFailureThreshold int    `json:"source_failure_threshold,omitempty"`
	SourceCooldown         string `json:"source_cooldown,omitempty"`
}

type CheckConfig struct {
	Workers               int      `json:"workers,omitempty"`
	TargetURL             string   `json:"target_url,omitempty"`
	TargetURLs            []string `json:"target_urls,omitempty"`
	BodyContains          string   `json:"body_contains,omitempty"`
	Timeout               string   `json:"timeout,omitempty"`
	ConnectTimeout        string   `json:"connect_timeout,omitempty"`
	TLSHandshakeTimeout   string   `json:"tls_handshake_timeout,omitempty"`
	ResponseHeaderTimeout string   `json:"response_header_timeout,omitempty"`
	IdleConnTimeout       string   `json:"idle_conn_timeout,omitempty"`
	MaxIdleConns          int      `json:"max_idle_conns,omitempty"`
	MaxIdleConnsPerHost   int      `json:"max_idle_conns_per_host,omitempty"`
}

type SchedulerConfig struct {
	Profile          string `json:"profile,omitempty"`
	MaxChecks        int    `json:"max_checks,omitempty"`
	CheckTTL         string `json:"check_ttl,omitempty"`
	HealthyCheckTTL  string `json:"healthy_check_ttl,omitempty"`
	DegradedCheckTTL string `json:"degraded_check_ttl,omitempty"`
	DeadCheckTTL     string `json:"dead_check_ttl,omitempty"`
	DeadBackoffMax   string `json:"dead_backoff_max,omitempty"`
	ProtocolFair     *bool  `json:"protocol_fair,omitempty"`
	SourceFair       *bool  `json:"source_fair,omitempty"`
	TailBiased       *bool  `json:"tail_biased,omitempty"`
	SkipUnsupported  *bool  `json:"skip_unsupported,omitempty"`
}

type RefreshConfig struct {
	Enabled            *bool   `json:"enabled,omitempty"`
	BaseInterval       string  `json:"base_interval,omitempty"`
	MinInterval        string  `json:"min_interval,omitempty"`
	MaxInterval        string  `json:"max_interval,omitempty"`
	Jitter             string  `json:"jitter,omitempty"`
	MinHealthy         int     `json:"min_healthy,omitempty"`
	MinHealthyRatio    float64 `json:"min_healthy_ratio,omitempty"`
	UncheckedThreshold int     `json:"unchecked_threshold,omitempty"`
	FailureBackoff     float64 `json:"failure_backoff,omitempty"`
}

type LoggingConfig struct {
	Level  string `json:"level,omitempty"`
	Format string `json:"format,omitempty"`
}

func DefaultAppConfig() AppConfig {
	cacheFallback := true
	refreshEnabled := true
	protocolFair := true
	sourceFair := true
	tailBiased := true
	skipUnsupported := true
	return AppConfig{
		Version:     1,
		SourcesPath: DefaultPath,
		Server: ServerConfig{
			Addr:            "127.0.0.1:8899",
			ShutdownTimeout: "10s",
		},
		Cache: CacheConfig{
			Path:     ".plugproxy.cache.json",
			Fallback: &cacheFallback,
		},
		Fetch: FetchConfig{
			SourceWorkers:          32,
			PerHostWorkers:         4,
			SourceFailureThreshold: 3,
			SourceCooldown:         "15m",
		},
		Check: CheckConfig{
			Workers:               32,
			TargetURL:             "https://httpbin.org/ip",
			Timeout:               "8s",
			ConnectTimeout:        "5s",
			TLSHandshakeTimeout:   "5s",
			ResponseHeaderTimeout: "5s",
			IdleConnTimeout:       "90s",
			MaxIdleConns:          256,
			MaxIdleConnsPerHost:   32,
		},
		Scheduler: SchedulerConfig{
			Profile:          "smart",
			HealthyCheckTTL:  "6h",
			DegradedCheckTTL: "30m",
			DeadCheckTTL:     "12h",
			DeadBackoffMax:   "72h",
			ProtocolFair:     &protocolFair,
			SourceFair:       &sourceFair,
			TailBiased:       &tailBiased,
			SkipUnsupported:  &skipUnsupported,
		},
		Refresh: RefreshConfig{
			Enabled:            &refreshEnabled,
			BaseInterval:       "5m",
			MinInterval:        "30s",
			MaxInterval:        "30m",
			Jitter:             "10s",
			MinHealthy:         1,
			UncheckedThreshold: 100,
			FailureBackoff:     2,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
		},
	}
}

func LoadApp(path string) (AppConfig, error) {
	if path == "" {
		path = DefaultAppPath
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return AppConfig{}, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, err
	}
	return cfg.WithDefaults(), nil
}

func SaveApp(path string, cfg AppConfig) error {
	if path == "" {
		path = DefaultAppPath
	}
	data, err := json.MarshalIndent(cfg.WithDefaults(), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func ValidateApp(cfg AppConfig) error {
	cfg = cfg.WithDefaults()
	durations := map[string]string{
		"server.shutdown_timeout":       cfg.Server.ShutdownTimeout,
		"fetch.source_cooldown":         cfg.Fetch.SourceCooldown,
		"check.timeout":                 cfg.Check.Timeout,
		"check.connect_timeout":         cfg.Check.ConnectTimeout,
		"check.tls_handshake_timeout":   cfg.Check.TLSHandshakeTimeout,
		"check.response_header_timeout": cfg.Check.ResponseHeaderTimeout,
		"check.idle_conn_timeout":       cfg.Check.IdleConnTimeout,
		"scheduler.check_ttl":           cfg.Scheduler.CheckTTL,
		"scheduler.healthy_check_ttl":   cfg.Scheduler.HealthyCheckTTL,
		"scheduler.degraded_check_ttl":  cfg.Scheduler.DegradedCheckTTL,
		"scheduler.dead_check_ttl":      cfg.Scheduler.DeadCheckTTL,
		"scheduler.dead_backoff_max":    cfg.Scheduler.DeadBackoffMax,
		"refresh.base_interval":         cfg.Refresh.BaseInterval,
		"refresh.min_interval":          cfg.Refresh.MinInterval,
		"refresh.max_interval":          cfg.Refresh.MaxInterval,
		"refresh.jitter":                cfg.Refresh.Jitter,
	}
	for name, value := range durations {
		if value == "" {
			continue
		}
		if _, err := time.ParseDuration(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func (c AppConfig) WithDefaults() AppConfig {
	defaults := DefaultAppConfig()
	if c.Version == 0 {
		c.Version = defaults.Version
	}
	if c.SourcesPath == "" {
		c.SourcesPath = defaults.SourcesPath
	}
	if c.Server.Addr == "" {
		c.Server.Addr = defaults.Server.Addr
	}
	if c.Server.ShutdownTimeout == "" {
		c.Server.ShutdownTimeout = defaults.Server.ShutdownTimeout
	}
	if c.Cache.Path == "" {
		c.Cache.Path = defaults.Cache.Path
	}
	if c.Cache.Fallback == nil {
		c.Cache.Fallback = defaults.Cache.Fallback
	}
	if c.Fetch.SourceWorkers == 0 {
		c.Fetch.SourceWorkers = defaults.Fetch.SourceWorkers
	}
	if c.Fetch.PerHostWorkers == 0 {
		c.Fetch.PerHostWorkers = defaults.Fetch.PerHostWorkers
	}
	if c.Fetch.SourceFailureThreshold == 0 {
		c.Fetch.SourceFailureThreshold = defaults.Fetch.SourceFailureThreshold
	}
	if c.Fetch.SourceCooldown == "" {
		c.Fetch.SourceCooldown = defaults.Fetch.SourceCooldown
	}
	if c.Check.Workers == 0 {
		c.Check.Workers = defaults.Check.Workers
	}
	if c.Check.TargetURL == "" {
		c.Check.TargetURL = defaults.Check.TargetURL
	}
	if c.Check.Timeout == "" {
		c.Check.Timeout = defaults.Check.Timeout
	}
	if c.Check.ConnectTimeout == "" {
		c.Check.ConnectTimeout = defaults.Check.ConnectTimeout
	}
	if c.Check.TLSHandshakeTimeout == "" {
		c.Check.TLSHandshakeTimeout = defaults.Check.TLSHandshakeTimeout
	}
	if c.Check.ResponseHeaderTimeout == "" {
		c.Check.ResponseHeaderTimeout = defaults.Check.ResponseHeaderTimeout
	}
	if c.Check.IdleConnTimeout == "" {
		c.Check.IdleConnTimeout = defaults.Check.IdleConnTimeout
	}
	if c.Check.MaxIdleConns == 0 {
		c.Check.MaxIdleConns = defaults.Check.MaxIdleConns
	}
	if c.Check.MaxIdleConnsPerHost == 0 {
		c.Check.MaxIdleConnsPerHost = defaults.Check.MaxIdleConnsPerHost
	}
	if c.Scheduler.Profile == "" {
		c.Scheduler.Profile = defaults.Scheduler.Profile
	}
	if c.Scheduler.HealthyCheckTTL == "" {
		c.Scheduler.HealthyCheckTTL = defaults.Scheduler.HealthyCheckTTL
	}
	if c.Scheduler.DegradedCheckTTL == "" {
		c.Scheduler.DegradedCheckTTL = defaults.Scheduler.DegradedCheckTTL
	}
	if c.Scheduler.DeadCheckTTL == "" {
		c.Scheduler.DeadCheckTTL = defaults.Scheduler.DeadCheckTTL
	}
	if c.Scheduler.DeadBackoffMax == "" {
		c.Scheduler.DeadBackoffMax = defaults.Scheduler.DeadBackoffMax
	}
	if c.Scheduler.ProtocolFair == nil {
		c.Scheduler.ProtocolFair = defaults.Scheduler.ProtocolFair
	}
	if c.Scheduler.SourceFair == nil {
		c.Scheduler.SourceFair = defaults.Scheduler.SourceFair
	}
	if c.Scheduler.TailBiased == nil {
		c.Scheduler.TailBiased = defaults.Scheduler.TailBiased
	}
	if c.Scheduler.SkipUnsupported == nil {
		c.Scheduler.SkipUnsupported = defaults.Scheduler.SkipUnsupported
	}
	if c.Refresh.Enabled == nil {
		c.Refresh.Enabled = defaults.Refresh.Enabled
	}
	if c.Refresh.BaseInterval == "" {
		c.Refresh.BaseInterval = defaults.Refresh.BaseInterval
	}
	if c.Refresh.MinInterval == "" {
		c.Refresh.MinInterval = defaults.Refresh.MinInterval
	}
	if c.Refresh.MaxInterval == "" {
		c.Refresh.MaxInterval = defaults.Refresh.MaxInterval
	}
	if c.Refresh.Jitter == "" {
		c.Refresh.Jitter = defaults.Refresh.Jitter
	}
	if c.Refresh.MinHealthy == 0 {
		c.Refresh.MinHealthy = defaults.Refresh.MinHealthy
	}
	if c.Refresh.UncheckedThreshold == 0 {
		c.Refresh.UncheckedThreshold = defaults.Refresh.UncheckedThreshold
	}
	if c.Refresh.FailureBackoff == 0 {
		c.Refresh.FailureBackoff = defaults.Refresh.FailureBackoff
	}
	if c.Logging.Level == "" {
		c.Logging.Level = defaults.Logging.Level
	}
	if c.Logging.Format == "" {
		c.Logging.Format = defaults.Logging.Format
	}
	return c
}

func Duration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func Bool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
