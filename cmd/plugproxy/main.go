package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/LING71671/plugproxy/internal/app"
	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/discover"
	"github.com/LING71671/plugproxy/internal/doctor"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/pkg/model"
)

var (
	version = "0.1.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "version":
		fmt.Printf("%s commit=%s date=%s\n", version, commit, date)
	case "init":
		fs := flag.NewFlagSet("init", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		force := fs.Bool("force", false, "overwrite existing config")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "force": true}))
		if err := writeInitialConfig(*configPath, *force); err != nil {
			exitErr(err)
		}
		writeJSON(map[string]any{"config": *configPath, "created": true})
	case "doctor":
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		apiURL := fs.String("api", "", "optional plugproxy API base URL")
		sourceCheck := fs.Bool("source-check", false, "fetch sources during diagnosis")
		sourceWorkers := fs.Int("source-workers", 8, "number of concurrent source checks")
		timeout := fs.Duration("timeout", 5*time.Second, "doctor request timeout")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"config": false, "cache": false, "api": false, "source-check": true,
			"source-workers": false, "timeout": false,
		}))
		report := doctor.Run(ctx, doctor.Options{
			ConfigPath:    *configPath,
			CachePath:     *cachePath,
			APIURL:        *apiURL,
			SourceCheck:   *sourceCheck,
			SourceWorkers: *sourceWorkers,
			Timeout:       *timeout,
		})
		writeJSON(report)
		if !report.OK {
			os.Exit(1)
		}
	case "fetch":
		fs := flag.NewFlagSet("fetch", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false}))
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		report := application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:       *sourceWorkers,
			CachePath:     *cachePath,
			CacheFallback: *cacheFallback,
			CacheWrite:    true,
		})
		writeJSON(report)
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		workers := fs.Int("workers", 32, "number of concurrent proxy checks")
		protocol := fs.String("protocol", "", "protocol filter: http, https, socks4, socks5")
		target := fs.String("target", "https://httpbin.org/ip", "target URL used to check proxies")
		timeout := fs.Duration("timeout", 8*time.Second, "per-proxy check timeout")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "workers": false, "protocol": false, "target": false, "timeout": false}))
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:       *sourceWorkers,
			CachePath:     *cachePath,
			CacheFallback: *cacheFallback,
			CacheWrite:    true,
		})
		stats := application.CheckWithOptions(ctx, app.CheckOptions{
			Workers:    *workers,
			TargetURL:  *target,
			Timeout:    *timeout,
			Filter:     pool.Filter{Protocol: model.Protocol(*protocol)},
			CachePath:  *cachePath,
			CacheWrite: true,
		})
		writeJSON(stats)
	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false}))
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:       *sourceWorkers,
			CachePath:     *cachePath,
			CacheFallback: *cacheFallback,
			CacheWrite:    true,
		})
		writeJSON(application.Pool().List(pool.Filter{}))
	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		strategy := fs.String("strategy", string(pool.StrategyAny), "selection strategy: any, fastest")
		protocol := fs.String("protocol", "", "protocol filter: http, https, socks4, socks5")
		healthy := fs.Bool("healthy", false, "only return healthy proxies")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "strategy": false, "protocol": false, "healthy": true}))
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:       *sourceWorkers,
			CachePath:     *cachePath,
			CacheFallback: *cacheFallback,
			CacheWrite:    true,
		})
		proxy, ok := application.Pool().Get(pool.Strategy(*strategy), pool.Filter{Protocol: model.Protocol(*protocol), Healthy: *healthy})
		if !ok {
			fmt.Fprintln(os.Stderr, "no proxy available")
			os.Exit(1)
		}
		writeJSON(proxy)
	case "stats":
		fs := flag.NewFlagSet("stats", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		fetch := fs.Bool("fetch", false, "fetch and merge sources before computing stats")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "fetch": true}))
		if *fetch {
			application, err := newApplication(log, *configPath)
			if err != nil {
				exitErr(err)
			}
			application.FetchWithOptions(ctx, app.FetchOptions{
				Workers:       *sourceWorkers,
				CachePath:     *cachePath,
				CacheFallback: *cacheFallback,
				CacheWrite:    true,
			})
			writeJSON(model.NewProxyStats(application.Pool().List(pool.Filter{})))
			break
		}
		proxies, err := cache.Load(*cachePath)
		if err != nil {
			exitErr(err)
		}
		writeJSON(model.NewProxyStats(proxies))
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		addr := fs.String("addr", "127.0.0.1:8899", "HTTP API listen address")
		workers := fs.Int("workers", 32, "number of concurrent proxy checks")
		target := fs.String("target", "https://httpbin.org/ip", "target URL used to check proxies")
		timeout := fs.Duration("timeout", 8*time.Second, "per-proxy check timeout")
		skipCheck := fs.Bool("skip-check", true, "skip proxy checking on startup")
		refresh := fs.Bool("refresh", true, "enable background fetch and check refresh")
		refreshInterval := fs.Duration("refresh-interval", 5*time.Minute, "background refresh interval")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "addr": false, "workers": false,
			"target": false, "timeout": false, "skip-check": true, "refresh": true, "refresh-interval": false,
		}))

		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:       *sourceWorkers,
			CachePath:     *cachePath,
			CacheFallback: *cacheFallback,
			CacheWrite:    true,
		})
		if !*skipCheck {
			application.CheckWithOptions(ctx, app.CheckOptions{
				Workers:    *workers,
				TargetURL:  *target,
				Timeout:    *timeout,
				CachePath:  *cachePath,
				CacheWrite: true,
			})
		}
		refreshOptions := app.RefreshOptions{
			Fetch: app.FetchOptions{
				Workers:       *sourceWorkers,
				CachePath:     *cachePath,
				CacheFallback: *cacheFallback,
				CacheWrite:    true,
			},
			Check: app.CheckOptions{
				Workers:    *workers,
				TargetURL:  *target,
				Timeout:    *timeout,
				CachePath:  *cachePath,
				CacheWrite: true,
			},
		}
		if *refresh {
			application.StartAutoRefresh(ctx, *refreshInterval, refreshOptions)
		}
		if err := application.ServeWithRefresh(*addr, refreshOptions); err != nil {
			log.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case "discover":
		if err := runDiscover(ctx, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(`plugproxy is a lightweight proxy crawling, checking, pooling, and integration toolkit.

Usage:
  plugproxy version
  plugproxy init [-config plugproxy.sources.json] [-force]
  plugproxy doctor [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-api http://127.0.0.1:8899] [-source-check=false]
  plugproxy fetch [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32]
  plugproxy check [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-workers 32] [-protocol http] [-target URL] [-timeout 8s]
  plugproxy list [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32]
  plugproxy get [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-strategy fastest] [-protocol http] [-healthy=true]
  plugproxy stats [-cache .plugproxy.cache.json] [-fetch=false]
  plugproxy run [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-addr 127.0.0.1:8899] [-skip-check=true] [-refresh=true] [-refresh-interval 5m]
  plugproxy discover repo owner/name
  plugproxy discover url URL
  plugproxy discover validate FILE
  plugproxy discover search -query QUERY [-ai]`)
}

func writeJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func newApplication(log *slog.Logger, configPath string) (*app.App, error) {
	sources, err := config.LoadSources(configPath)
	if err != nil {
		return nil, err
	}
	return app.NewWithSources(log, sources), nil
}

func writeInitialConfig(path string, force bool) error {
	if path == "" {
		path = config.DefaultPath
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; use -force to overwrite", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return config.Save(path, config.DefaultConfig())
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func runDiscover(ctx context.Context, args []string) error {
	if len(args) < 1 {
		discoverUsage()
		return fmt.Errorf("missing discover command")
	}

	switch args[0] {
	case "repo":
		fs := flag.NewFlagSet("discover repo", flag.ExitOnError)
		timeout := fs.Duration("timeout", 15*time.Second, "GitHub request timeout")
		workers := fs.Int("workers", 16, "concurrent GitHub file scans")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"timeout": false, "workers": false}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy discover repo owner/name")
		}
		client := discover.NewGitHubClient(*timeout, *workers)
		writeJSON(client.DiscoverRepo(ctx, fs.Arg(0)))
	case "url":
		fs := flag.NewFlagSet("discover url", flag.ExitOnError)
		timeout := fs.Duration("timeout", 12*time.Second, "URL request timeout")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"timeout": false}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy discover url URL")
		}
		httpClient := discover.NewHTTPClient(*timeout, 0)
		content, err := httpClient.FetchSample(ctx, fs.Arg(0))
		if err != nil {
			return err
		}
		report := discover.NewReport(fs.Arg(0), "url")
		report.Candidates = discover.NewAnalyzer().AnalyzeURLContent(fs.Arg(0), content, "url:"+fs.Arg(0))
		writeJSON(report)
	case "validate":
		fs := flag.NewFlagSet("discover validate", flag.ExitOnError)
		timeout := fs.Duration("timeout", 12*time.Second, "per-source validation timeout")
		workers := fs.Int("workers", 128, "concurrent source validations")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"timeout": false, "workers": false}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy discover validate FILE")
		}
		report, err := readDiscoveryInput(fs.Arg(0))
		if err != nil {
			return err
		}
		report.Source = "validate"
		report.Generated = time.Now()
		report.Candidates = discover.NewValidator(*timeout, *workers).Validate(ctx, report.Candidates)
		writeJSON(report)
	case "search":
		fs := flag.NewFlagSet("discover search", flag.ExitOnError)
		query := fs.String("query", "", "search query")
		limit := fs.Int("limit", 10, "maximum results")
		useAI := fs.Bool("ai", false, "enable AI web search")
		aiProviderName := fs.String("ai-provider", "openai", "AI provider: openai or responses-compatible")
		aiModel := fs.String("ai-model", "gpt-5", "AI model")
		aiBaseURL := fs.String("ai-base-url", os.Getenv("PLUGPROXY_AI_BASE_URL"), "Responses-compatible API base URL")
		timeout := fs.Duration("timeout", 45*time.Second, "search request timeout")
		workers := fs.Int("workers", 16, "concurrent GitHub search enrichment workers")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{
			"query": false, "limit": false, "ai": true, "ai-provider": false,
			"ai-model": false, "ai-base-url": false, "timeout": false, "workers": false,
		}))
		if *query == "" {
			return fmt.Errorf("discover search requires -query")
		}
		report := discover.NewGitHubClient(*timeout, *workers).SearchRepos(ctx, *query, *limit)
		if *useAI {
			provider, err := discover.NewAIProvider(*aiProviderName, *aiModel, *aiBaseURL, *timeout)
			if err != nil {
				return err
			}
			aiReport, err := provider.Search(ctx, *query, *limit)
			if err != nil {
				report.Failures = append(report.Failures, err.Error())
			} else {
				report.Candidates = append(report.Candidates, aiReport.Candidates...)
			}
			report.Source = "github_search+ai"
			report.Candidates = discover.Deduplicate(report.Candidates)
		}
		writeJSON(report)
	default:
		discoverUsage()
		return fmt.Errorf("unknown discover command %q", args[0])
	}
	return nil
}

func discoverUsage() {
	fmt.Println(`Discover commands:
  plugproxy discover repo owner/name
  plugproxy discover url URL
  plugproxy discover validate FILE
  plugproxy discover search -query QUERY [-limit 10] [-workers 16] [-ai] [-ai-provider openai] [-ai-model gpt-5]`)
}

func readDiscoveryInput(path string) (discover.DiscoveryReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return discover.DiscoveryReport{}, err
	}

	var report discover.DiscoveryReport
	if err := json.Unmarshal(data, &report); err == nil && len(report.Candidates) > 0 {
		return report, nil
	}

	var candidates []discover.CandidateSource
	if err := json.Unmarshal(data, &candidates); err != nil {
		return discover.DiscoveryReport{}, err
	}
	report = discover.NewReport(path, "file")
	report.Candidates = candidates
	return report, nil
}

func reorderFlagArgs(args []string, boolFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}

		nameValue := strings.TrimLeft(arg, "-")
		name := nameValue
		if index := strings.Index(nameValue, "="); index >= 0 {
			name = nameValue[:index]
		}
		flags = append(flags, arg)
		if strings.Contains(nameValue, "=") || boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positionals...)
}
