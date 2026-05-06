package doctor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/fetcher"
	"github.com/LING71671/plugproxy/pkg/model"
)

const (
	StatusOK   = "ok"
	StatusWarn = "warn"
	StatusFail = "fail"
)

type Options struct {
	ConfigPath    string
	CachePath     string
	APIURL        string
	SourceCheck   bool
	SourceWorkers int
	Timeout       time.Duration
}

type Report struct {
	OK      bool          `json:"ok"`
	Checks  []Check       `json:"checks"`
	Config  ConfigReport  `json:"config"`
	Cache   CacheReport   `json:"cache"`
	API     APIReport     `json:"api,omitempty"`
	Sources SourcesReport `json:"sources,omitempty"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ConfigReport struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Sources int    `json:"sources"`
	Enabled int    `json:"enabled"`
}

type CacheReport struct {
	Path    string           `json:"path"`
	Exists  bool             `json:"exists"`
	Proxies int              `json:"proxies"`
	Stats   model.ProxyStats `json:"stats,omitempty"`
}

type APIReport struct {
	URL    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

type SourcesReport struct {
	Checked int                 `json:"checked"`
	OK      int                 `json:"ok"`
	Failed  int                 `json:"failed"`
	Proxies int                 `json:"proxies"`
	Items   []SourceCheckReport `json:"items,omitempty"`
}

type SourceCheckReport struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

func Run(ctx context.Context, options Options) Report {
	if options.ConfigPath == "" {
		options.ConfigPath = config.DefaultPath
	}
	if options.CachePath == "" {
		options.CachePath = cache.DefaultPath
	}
	if options.Timeout <= 0 {
		options.Timeout = 5 * time.Second
	}
	if options.SourceWorkers <= 0 {
		options.SourceWorkers = 8
	}

	report := Report{OK: true}
	cfg, cfgOK := checkConfig(&report, options.ConfigPath)
	checkCache(&report, options.CachePath)
	if options.APIURL != "" {
		checkAPI(ctx, &report, options.APIURL, options.Timeout)
	}
	if options.SourceCheck && cfgOK {
		checkSources(ctx, &report, cfg, options.SourceWorkers)
	}
	return report
}

func checkConfig(report *Report, path string) (config.Config, bool) {
	cfg, err := config.Load(path)
	exists := true
	if _, statErr := os.Stat(path); statErr != nil {
		exists = false
	}
	report.Config.Path = path
	report.Config.Exists = exists
	if err != nil {
		report.add("config", StatusFail, err.Error())
		return config.Config{}, false
	}
	report.Config.Sources = len(cfg.Sources)
	for _, source := range cfg.Sources {
		if source.Enabled == nil || *source.Enabled {
			report.Config.Enabled++
		}
	}
	report.add("config", StatusOK, fmt.Sprintf("%d enabled sources", report.Config.Enabled))
	return cfg, true
}

func checkCache(report *Report, path string) {
	report.Cache.Path = path
	proxies, err := cache.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.add("cache", StatusWarn, "cache file does not exist yet")
			return
		}
		report.add("cache", StatusFail, err.Error())
		return
	}
	report.Cache.Exists = true
	report.Cache.Proxies = len(proxies)
	report.Cache.Stats = model.NewProxyStats(proxies)
	report.add("cache", StatusOK, fmt.Sprintf("%d cached proxies", len(proxies)))
}

func checkAPI(ctx context.Context, report *Report, apiURL string, timeout time.Duration) {
	report.API.URL = apiURL
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, apiURL+"/health", nil)
	if err != nil {
		report.add("api", StatusWarn, err.Error())
		report.API.Status = StatusWarn
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		report.add("api", StatusWarn, err.Error())
		report.API.Status = StatusWarn
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		report.add("api", StatusOK, "api health endpoint is reachable")
		report.API.Status = StatusOK
		return
	}
	report.add("api", StatusWarn, resp.Status)
	report.API.Status = StatusWarn
}

func checkSources(ctx context.Context, report *Report, cfg config.Config, workers int) {
	sources, err := config.BuildSources(cfg)
	if err != nil {
		report.add("sources", StatusFail, err.Error())
		return
	}
	results := fetcher.FetchAllWithWorkers(ctx, sources, workers)
	sourceConfigs := enabledSourceConfigs(cfg)
	report.Sources.Checked = len(results)
	report.Sources.Items = make([]SourceCheckReport, 0, len(results))
	for index, result := range results {
		item := SourceCheckReport{
			Name:   result.Source,
			Type:   sourceTypeAt(sourceConfigs, index),
			Status: StatusOK,
			Count:  len(result.Proxies),
		}
		if result.Error != nil {
			report.Sources.Failed++
			item.Status = StatusFail
			item.Error = result.Error.Error()
			report.Sources.Items = append(report.Sources.Items, item)
			continue
		}
		report.Sources.OK++
		report.Sources.Proxies += len(result.Proxies)
		report.Sources.Items = append(report.Sources.Items, item)
	}
	if report.Sources.Failed > 0 {
		report.add("sources", StatusWarn, fmt.Sprintf("%d/%d sources failed", report.Sources.Failed, report.Sources.Checked))
		return
	}
	report.add("sources", StatusOK, fmt.Sprintf("%d proxies fetched", report.Sources.Proxies))
}

func enabledSourceConfigs(cfg config.Config) []config.SourceConfig {
	items := make([]config.SourceConfig, 0, len(cfg.Sources))
	for _, source := range cfg.Sources {
		if source.Enabled != nil && !*source.Enabled {
			continue
		}
		items = append(items, source)
	}
	return items
}

func sourceTypeAt(sources []config.SourceConfig, index int) string {
	if index < 0 || index >= len(sources) {
		return ""
	}
	if sources[index].Type == "" {
		return "raw_text_url"
	}
	return sources[index].Type
}

func (r *Report) add(name string, status string, message string) {
	if status == StatusFail {
		r.OK = false
	}
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Message: message})
}
