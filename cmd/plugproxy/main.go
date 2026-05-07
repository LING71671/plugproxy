package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"github.com/LING71671/plugproxy/internal/app"
	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/checker"
	"github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/discover"
	"github.com/LING71671/plugproxy/internal/doctor"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/scheduler"
	"github.com/LING71671/plugproxy/internal/source"
	"github.com/LING71671/plugproxy/pkg/client"
	"github.com/LING71671/plugproxy/pkg/model"
)

var (
	version = "v0.5.1"
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
	case "config":
		if err := runConfigCommand(os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "cache":
		if err := runCacheCommand(os.Args[2:]); err != nil {
			exitErr(err)
		}
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
		appCfg, appConfigPath, _, err := loadAppDefaults(os.Args[2:])
		if err != nil {
			exitErr(err)
		}
		fs := flag.NewFlagSet("doctor", flag.ExitOnError)
		appConfigFlag := fs.String("app-config", appConfigPath, "app config path")
		configPath := fs.String("config", appCfg.SourcesPath, "source config path")
		cachePath := fs.String("cache", appCfg.Cache.Path, "proxy cache path")
		apiURL := fs.String("api", "", "optional plugproxy API base URL")
		sourceCheck := fs.Bool("source-check", false, "fetch sources during diagnosis")
		sourceWorkers := fs.Int("source-workers", appCfg.Fetch.SourceWorkers, "number of concurrent source checks")
		timeout := fs.Duration("timeout", config.Duration(appCfg.Check.Timeout, 5*time.Second), "doctor request timeout")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"app-config": false, "config": false, "cache": false, "api": false, "source-check": true,
			"source-workers": false, "timeout": false,
		}))
		_ = appConfigFlag
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
		appCfg, appConfigPath, _, err := loadAppDefaults(os.Args[2:])
		if err != nil {
			exitErr(err)
		}
		fs := flag.NewFlagSet("fetch", flag.ExitOnError)
		appConfigFlag := fs.String("app-config", appConfigPath, "app config path")
		configPath := fs.String("config", appCfg.SourcesPath, "source config path")
		cachePath := fs.String("cache", appCfg.Cache.Path, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", config.Bool(appCfg.Cache.Fallback, true), "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", appCfg.Fetch.SourceWorkers, "number of concurrent source fetches")
		perHostWorkers := fs.Int("per-host-workers", appCfg.Fetch.PerHostWorkers, "maximum concurrent source fetches per host")
		sourceFailureThreshold := fs.Int("source-failure-threshold", appCfg.Fetch.SourceFailureThreshold, "consecutive source failures before cooldown")
		sourceCooldown := fs.Duration("source-cooldown", config.Duration(appCfg.Fetch.SourceCooldown, 15*time.Minute), "source cooldown after repeated failures")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"app-config": false, "config": false, "cache": false, "cache-fallback": true, "source-workers": false,
			"per-host-workers": false, "source-failure-threshold": false, "source-cooldown": false,
		}))
		_ = appConfigFlag
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		report := application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:                *sourceWorkers,
			PerHostWorkers:         *perHostWorkers,
			SourceFailureThreshold: *sourceFailureThreshold,
			SourceCooldown:         *sourceCooldown,
			CachePath:              *cachePath,
			CacheFallback:          *cacheFallback,
			CacheWrite:             true,
		})
		writeJSON(report)
	case "check":
		appCfg, appConfigPath, appConfigLoaded, err := loadAppDefaults(os.Args[2:])
		if err != nil {
			exitErr(err)
		}
		checkProfileDefault := "full"
		protocolFairDefault := false
		sourceFairDefault := false
		tailBiasedDefault := false
		skipUnsupportedDefault := false
		if appConfigLoaded {
			checkProfileDefault = appCfg.Scheduler.Profile
			protocolFairDefault = config.Bool(appCfg.Scheduler.ProtocolFair, false)
			sourceFairDefault = config.Bool(appCfg.Scheduler.SourceFair, false)
			tailBiasedDefault = config.Bool(appCfg.Scheduler.TailBiased, false)
			skipUnsupportedDefault = config.Bool(appCfg.Scheduler.SkipUnsupported, false)
		}
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		appConfigFlag := fs.String("app-config", appConfigPath, "app config path")
		configPath := fs.String("config", appCfg.SourcesPath, "source config path")
		cachePath := fs.String("cache", appCfg.Cache.Path, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", config.Bool(appCfg.Cache.Fallback, true), "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", appCfg.Fetch.SourceWorkers, "number of concurrent source fetches")
		perHostWorkers := fs.Int("per-host-workers", appCfg.Fetch.PerHostWorkers, "maximum concurrent source fetches per host")
		sourceFailureThreshold := fs.Int("source-failure-threshold", appCfg.Fetch.SourceFailureThreshold, "consecutive source failures before cooldown")
		sourceCooldown := fs.Duration("source-cooldown", config.Duration(appCfg.Fetch.SourceCooldown, 15*time.Minute), "source cooldown after repeated failures")
		workers := fs.Int("workers", appCfg.Check.Workers, "number of concurrent proxy checks")
		protocol := fs.String("protocol", "", "protocol filter: http, https, socks4, socks5")
		target := fs.String("target", appCfg.Check.TargetURL, "target URL used to check proxies")
		targetHTTP := fs.String("target-http", "", "additional HTTP target URL used to check proxies")
		targetHTTPS := fs.String("target-https", "", "additional HTTPS target URL used to check proxies")
		targetFallbacks := fs.String("target-fallbacks", "", "comma-separated fallback target URLs")
		bodyContains := fs.String("body-contains", appCfg.Check.BodyContains, "optional substring that a successful target response body must contain")
		timeout := fs.Duration("timeout", config.Duration(appCfg.Check.Timeout, 8*time.Second), "per-proxy check timeout")
		maxChecks := fs.Int("max-checks", appCfg.Scheduler.MaxChecks, "maximum proxies to check in this run; 0 means unlimited")
		checkTTL := fs.Duration("check-ttl", config.Duration(appCfg.Scheduler.CheckTTL, 0), "skip proxies checked within this duration; 0 disables skipping")
		checkProfile := fs.String("check-profile", checkProfileDefault, "check scheduling profile: full or smart")
		healthyCheckTTL := fs.Duration("healthy-check-ttl", config.Duration(appCfg.Scheduler.HealthyCheckTTL, 6*time.Hour), "smart profile TTL for healthy proxies")
		degradedCheckTTL := fs.Duration("degraded-check-ttl", config.Duration(appCfg.Scheduler.DegradedCheckTTL, 30*time.Minute), "smart profile TTL for degraded proxies")
		deadCheckTTL := fs.Duration("dead-check-ttl", config.Duration(appCfg.Scheduler.DeadCheckTTL, 12*time.Hour), "smart profile TTL for dead proxies")
		deadBackoffMax := fs.Duration("dead-backoff-max", config.Duration(appCfg.Scheduler.DeadBackoffMax, 72*time.Hour), "maximum smart profile dead proxy backoff")
		protocolFair := fs.Bool("protocol-fair", protocolFairDefault, "distribute limited checks across protocols")
		sourceFair := fs.Bool("source-fair", sourceFairDefault, "distribute limited checks across sources")
		tailBiased := fs.Bool("tail-biased", tailBiasedDefault, "prefer later entries from each source when source fair sampling is enabled")
		skipUnsupported := fs.Bool("skip-unsupported", skipUnsupportedDefault, "skip protocols that the checker cannot support")
		connectTimeout := fs.Duration("connect-timeout", config.Duration(appCfg.Check.ConnectTimeout, 5*time.Second), "proxy connection timeout")
		tlsHandshakeTimeout := fs.Duration("tls-handshake-timeout", config.Duration(appCfg.Check.TLSHandshakeTimeout, 5*time.Second), "TLS handshake timeout")
		responseHeaderTimeout := fs.Duration("response-header-timeout", config.Duration(appCfg.Check.ResponseHeaderTimeout, 5*time.Second), "response header timeout")
		idleConnTimeout := fs.Duration("idle-conn-timeout", config.Duration(appCfg.Check.IdleConnTimeout, 90*time.Second), "idle connection timeout")
		maxIdleConns := fs.Int("max-idle-conns", appCfg.Check.MaxIdleConns, "maximum idle connections")
		maxIdleConnsPerHost := fs.Int("max-idle-conns-per-host", appCfg.Check.MaxIdleConnsPerHost, "maximum idle connections per host")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"app-config": false, "config": false, "cache": false, "cache-fallback": true, "source-workers": false,
			"per-host-workers": false, "source-failure-threshold": false, "source-cooldown": false,
			"workers": false, "protocol": false, "target": false, "target-http": false, "target-https": false,
			"target-fallbacks": false, "body-contains": false, "timeout": false, "max-checks": false, "check-ttl": false,
			"check-profile": false, "healthy-check-ttl": false, "degraded-check-ttl": false, "dead-check-ttl": false,
			"dead-backoff-max": false, "protocol-fair": true, "source-fair": true, "tail-biased": true, "skip-unsupported": true,
			"connect-timeout": false, "tls-handshake-timeout": false, "response-header-timeout": false,
			"idle-conn-timeout": false, "max-idle-conns": false, "max-idle-conns-per-host": false,
		}))
		_ = appConfigFlag
		if schedulerProfile(*checkProfile) == scheduler.ProfileSmart {
			if !flagWasSet(fs, "protocol-fair") {
				*protocolFair = true
			}
			if !flagWasSet(fs, "source-fair") {
				*sourceFair = true
			}
			if !flagWasSet(fs, "tail-biased") {
				*tailBiased = true
			}
			if !flagWasSet(fs, "skip-unsupported") {
				*skipUnsupported = true
			}
		}
		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		application.FetchWithOptions(ctx, app.FetchOptions{
			Workers:                *sourceWorkers,
			PerHostWorkers:         *perHostWorkers,
			SourceFailureThreshold: *sourceFailureThreshold,
			SourceCooldown:         *sourceCooldown,
			CachePath:              *cachePath,
			CacheFallback:          *cacheFallback,
			CacheWrite:             true,
		})
		stats := application.CheckWithOptions(ctx, app.CheckOptions{
			Workers:          *workers,
			TargetURL:        *target,
			TargetURLs:       targetURLs(*target, *targetHTTP, *targetHTTPS, *targetFallbacks, appCfg.Check.TargetURLs),
			BodyContains:     *bodyContains,
			Timeout:          *timeout,
			Filter:           pool.Filter{Protocol: model.Protocol(*protocol)},
			CachePath:        *cachePath,
			CacheWrite:       true,
			MaxChecks:        *maxChecks,
			CheckTTL:         *checkTTL,
			Profile:          schedulerProfile(*checkProfile),
			HealthyCheckTTL:  *healthyCheckTTL,
			DegradedCheckTTL: *degradedCheckTTL,
			DeadCheckTTL:     *deadCheckTTL,
			DeadBackoffMax:   *deadBackoffMax,
			ProtocolFair:     *protocolFair,
			SourceFair:       *sourceFair,
			TailBiased:       *tailBiased,
			SkipUnsupported:  *skipUnsupported,
			Transport: checker.TransportOptions{
				ConnectTimeout:        *connectTimeout,
				TLSHandshakeTimeout:   *tlsHandshakeTimeout,
				ResponseHeaderTimeout: *responseHeaderTimeout,
				IdleConnTimeout:       *idleConnTimeout,
				MaxIdleConns:          *maxIdleConns,
				MaxIdleConnsPerHost:   *maxIdleConnsPerHost,
			},
		})
		writeJSON(stats)
	case "list":
		fs := flag.NewFlagSet("list", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		excludeDead := fs.Bool("exclude-dead", false, "exclude dead proxies from the list output")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "exclude-dead": true}))
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
		writeJSON(application.Pool().List(pool.Filter{ExcludeDead: *excludeDead}))
	case "get":
		fs := flag.NewFlagSet("get", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", true, "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		strategy := fs.String("strategy", string(pool.StrategyAny), "selection strategy: any, fastest, random, round_robin, least_recently_used, weighted")
		protocol := fs.String("protocol", "", "protocol filter: http, https, socks4, socks5")
		healthy := fs.Bool("healthy", false, "only return healthy proxies")
		excludeDead := fs.Bool("exclude-dead", true, "exclude dead proxies from selection")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{"config": false, "cache": false, "cache-fallback": true, "source-workers": false, "strategy": false, "protocol": false, "healthy": true, "exclude-dead": true}))
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
		proxy, ok := application.Pool().Get(pool.Strategy(*strategy), pool.Filter{Protocol: model.Protocol(*protocol), Healthy: *healthy, ExcludeDead: *excludeDead})
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
	case "watch":
		if err := runWatch(ctx, os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "sources":
		if err := runSourcesCommand(ctx, os.Args[2:]); err != nil {
			exitErr(err)
		}
	case "run":
		appCfg, appConfigPath, _, err := loadAppDefaults(os.Args[2:])
		if err != nil {
			exitErr(err)
		}
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		appConfigFlag := fs.String("app-config", appConfigPath, "app config path")
		configPath := fs.String("config", appCfg.SourcesPath, "source config path")
		cachePath := fs.String("cache", appCfg.Cache.Path, "proxy cache path")
		cacheFallback := fs.Bool("cache-fallback", config.Bool(appCfg.Cache.Fallback, true), "reuse proxy cache when all sources fail")
		sourceWorkers := fs.Int("source-workers", appCfg.Fetch.SourceWorkers, "number of concurrent source fetches")
		perHostWorkers := fs.Int("per-host-workers", appCfg.Fetch.PerHostWorkers, "maximum concurrent source fetches per host")
		addr := fs.String("addr", appCfg.Server.Addr, "HTTP API listen address")
		workers := fs.Int("workers", appCfg.Check.Workers, "number of concurrent proxy checks")
		target := fs.String("target", appCfg.Check.TargetURL, "target URL used to check proxies")
		targetHTTP := fs.String("target-http", "", "additional HTTP target URL used to check proxies")
		targetHTTPS := fs.String("target-https", "", "additional HTTPS target URL used to check proxies")
		targetFallbacks := fs.String("target-fallbacks", "", "comma-separated fallback target URLs")
		bodyContains := fs.String("body-contains", appCfg.Check.BodyContains, "optional substring that a successful target response body must contain")
		timeout := fs.Duration("timeout", config.Duration(appCfg.Check.Timeout, 8*time.Second), "per-proxy check timeout")
		maxChecks := fs.Int("max-checks", appCfg.Scheduler.MaxChecks, "maximum proxies to check per run; 0 means unlimited")
		checkTTL := fs.Duration("check-ttl", config.Duration(appCfg.Scheduler.CheckTTL, 0), "skip proxies checked within this duration; 0 disables skipping")
		checkProfile := fs.String("check-profile", appCfg.Scheduler.Profile, "check scheduling profile: full or smart")
		healthyCheckTTL := fs.Duration("healthy-check-ttl", config.Duration(appCfg.Scheduler.HealthyCheckTTL, 6*time.Hour), "smart profile TTL for healthy proxies")
		degradedCheckTTL := fs.Duration("degraded-check-ttl", config.Duration(appCfg.Scheduler.DegradedCheckTTL, 30*time.Minute), "smart profile TTL for degraded proxies")
		deadCheckTTL := fs.Duration("dead-check-ttl", config.Duration(appCfg.Scheduler.DeadCheckTTL, 12*time.Hour), "smart profile TTL for dead proxies")
		deadBackoffMax := fs.Duration("dead-backoff-max", config.Duration(appCfg.Scheduler.DeadBackoffMax, 72*time.Hour), "maximum smart profile dead proxy backoff")
		protocolFair := fs.Bool("protocol-fair", config.Bool(appCfg.Scheduler.ProtocolFair, true), "distribute limited checks across protocols")
		sourceFair := fs.Bool("source-fair", config.Bool(appCfg.Scheduler.SourceFair, true), "distribute limited checks across sources")
		tailBiased := fs.Bool("tail-biased", config.Bool(appCfg.Scheduler.TailBiased, true), "prefer later entries from each source when source fair sampling is enabled")
		skipUnsupported := fs.Bool("skip-unsupported", config.Bool(appCfg.Scheduler.SkipUnsupported, true), "skip protocols that the checker cannot support")
		skipCheck := fs.Bool("skip-check", true, "skip proxy checking on startup")
		refresh := fs.Bool("refresh", config.Bool(appCfg.Refresh.Enabled, true), "enable background fetch and check refresh")
		refreshInterval := fs.Duration("refresh-interval", config.Duration(appCfg.Refresh.BaseInterval, 5*time.Minute), "background refresh interval")
		refreshMinInterval := fs.Duration("refresh-min-interval", config.Duration(appCfg.Refresh.MinInterval, 30*time.Second), "minimum dynamic refresh interval")
		refreshMaxInterval := fs.Duration("refresh-max-interval", config.Duration(appCfg.Refresh.MaxInterval, 30*time.Minute), "maximum dynamic refresh interval")
		refreshJitter := fs.Duration("refresh-jitter", config.Duration(appCfg.Refresh.Jitter, 10*time.Second), "maximum random refresh delay jitter")
		minHealthy := fs.Int("min-healthy", appCfg.Refresh.MinHealthy, "refresh early when healthy proxy count is below this value")
		minHealthyRatio := fs.Float64("min-healthy-ratio", appCfg.Refresh.MinHealthyRatio, "refresh early when healthy proxy ratio is below this value")
		uncheckedThreshold := fs.Int("unchecked-threshold", appCfg.Refresh.UncheckedThreshold, "refresh early when unchecked proxy count reaches this value")
		refreshFailureBackoff := fs.Float64("refresh-failure-backoff", appCfg.Refresh.FailureBackoff, "multiply refresh delay after failed refresh")
		sourceFailureThreshold := fs.Int("source-failure-threshold", appCfg.Fetch.SourceFailureThreshold, "consecutive source failures before cooldown")
		sourceCooldown := fs.Duration("source-cooldown", config.Duration(appCfg.Fetch.SourceCooldown, 15*time.Minute), "source cooldown after repeated failures")
		connectTimeout := fs.Duration("connect-timeout", config.Duration(appCfg.Check.ConnectTimeout, 5*time.Second), "proxy connection timeout")
		tlsHandshakeTimeout := fs.Duration("tls-handshake-timeout", config.Duration(appCfg.Check.TLSHandshakeTimeout, 5*time.Second), "TLS handshake timeout")
		responseHeaderTimeout := fs.Duration("response-header-timeout", config.Duration(appCfg.Check.ResponseHeaderTimeout, 5*time.Second), "response header timeout")
		idleConnTimeout := fs.Duration("idle-conn-timeout", config.Duration(appCfg.Check.IdleConnTimeout, 90*time.Second), "idle connection timeout")
		maxIdleConns := fs.Int("max-idle-conns", appCfg.Check.MaxIdleConns, "maximum idle connections")
		maxIdleConnsPerHost := fs.Int("max-idle-conns-per-host", appCfg.Check.MaxIdleConnsPerHost, "maximum idle connections per host")
		shutdownTimeout := fs.Duration("shutdown-timeout", config.Duration(appCfg.Server.ShutdownTimeout, 10*time.Second), "graceful shutdown timeout")
		logLevel := fs.String("log-level", appCfg.Logging.Level, "log level: debug, info, warn, error")
		logFormat := fs.String("log-format", appCfg.Logging.Format, "log format: text or json")
		_ = fs.Parse(reorderFlagArgs(os.Args[2:], map[string]bool{
			"app-config": false, "config": false, "cache": false, "cache-fallback": true, "source-workers": false, "addr": false, "workers": false,
			"target": false, "target-http": false, "target-https": false, "target-fallbacks": false,
			"body-contains": false, "timeout": false, "max-checks": false, "check-ttl": false, "skip-check": true, "refresh": true, "refresh-interval": false,
			"refresh-min-interval": false, "refresh-max-interval": false, "refresh-jitter": false, "min-healthy": false,
			"min-healthy-ratio": false, "unchecked-threshold": false, "refresh-failure-backoff": false,
			"per-host-workers": false, "source-failure-threshold": false, "source-cooldown": false,
			"check-profile": false, "healthy-check-ttl": false, "degraded-check-ttl": false, "dead-check-ttl": false,
			"dead-backoff-max": false, "protocol-fair": true, "source-fair": true, "tail-biased": true, "skip-unsupported": true,
			"connect-timeout": false, "tls-handshake-timeout": false, "response-header-timeout": false,
			"idle-conn-timeout": false, "max-idle-conns": false, "max-idle-conns-per-host": false,
			"shutdown-timeout": false, "log-level": false, "log-format": false,
		}))
		_ = appConfigFlag
		log = newLogger(*logLevel, *logFormat)
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer stop()

		application, err := newApplication(log, *configPath)
		if err != nil {
			exitErr(err)
		}
		fetchOptions := app.FetchOptions{
			Workers:                *sourceWorkers,
			PerHostWorkers:         *perHostWorkers,
			SourceFailureThreshold: *sourceFailureThreshold,
			SourceCooldown:         *sourceCooldown,
			CachePath:              *cachePath,
			CacheFallback:          *cacheFallback,
			CacheWrite:             true,
		}
		checkOptions := app.CheckOptions{
			Workers:          *workers,
			TargetURL:        *target,
			TargetURLs:       targetURLs(*target, *targetHTTP, *targetHTTPS, *targetFallbacks, appCfg.Check.TargetURLs),
			BodyContains:     *bodyContains,
			Timeout:          *timeout,
			CachePath:        *cachePath,
			CacheWrite:       true,
			MaxChecks:        *maxChecks,
			CheckTTL:         *checkTTL,
			Profile:          schedulerProfile(*checkProfile),
			HealthyCheckTTL:  *healthyCheckTTL,
			DegradedCheckTTL: *degradedCheckTTL,
			DeadCheckTTL:     *deadCheckTTL,
			DeadBackoffMax:   *deadBackoffMax,
			ProtocolFair:     *protocolFair,
			SourceFair:       *sourceFair,
			TailBiased:       *tailBiased,
			SkipUnsupported:  *skipUnsupported,
			Transport: checker.TransportOptions{
				ConnectTimeout:        *connectTimeout,
				TLSHandshakeTimeout:   *tlsHandshakeTimeout,
				ResponseHeaderTimeout: *responseHeaderTimeout,
				IdleConnTimeout:       *idleConnTimeout,
				MaxIdleConns:          *maxIdleConns,
				MaxIdleConnsPerHost:   *maxIdleConnsPerHost,
			},
		}
		log.Info("effective config",
			"addr", *addr,
			"cache", *cachePath,
			"source_workers", *sourceWorkers,
			"per_host_workers", *perHostWorkers,
			"check_workers", *workers,
			"check_profile", *checkProfile,
			"max_checks", *maxChecks,
			"refresh", *refresh,
			"refresh_interval", *refreshInterval,
		)
		if *skipCheck {
			application.FetchWithOptions(ctx, fetchOptions)
		} else {
			application.FetchCheckWithOptions(ctx, fetchOptions, checkOptions)
		}
		refreshOptions := app.RefreshOptions{
			Fetch: fetchOptions,
			Check: checkOptions,
			Policy: app.RefreshPolicy{
				Enabled:            *refresh,
				BaseInterval:       *refreshInterval,
				MinInterval:        *refreshMinInterval,
				MaxInterval:        *refreshMaxInterval,
				Jitter:             *refreshJitter,
				MinHealthy:         *minHealthy,
				MinHealthyRatio:    *minHealthyRatio,
				UncheckedThreshold: *uncheckedThreshold,
				FailureBackoff:     *refreshFailureBackoff,
			},
		}
		if *refresh {
			application.StartAutoRefresh(ctx, *refreshInterval, refreshOptions)
		}
		server := &http.Server{Addr: *addr, Handler: application.Handler(refreshOptions)}
		errCh := make(chan error, 1)
		go func() {
			log.Info("api server listening", "addr", *addr)
			errCh <- server.ListenAndServe()
		}()
		select {
		case err := <-errCh:
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("server stopped", "error", err)
				os.Exit(1)
			}
		case <-ctx.Done():
			log.Info("shutdown requested")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Error("server shutdown failed", "error", err)
				os.Exit(1)
			}
			if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("server stopped", "error", err)
				os.Exit(1)
			}
			log.Info("shutdown completed")
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
  plugproxy config init|validate|print [-app-config plugproxy.config.json]
  plugproxy cache stats|compact|repair [-cache .plugproxy.cache.json]
  plugproxy sources list|validate|test|add|remove|enable|disable|export
  plugproxy doctor [-app-config plugproxy.config.json] [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-api http://127.0.0.1:8899] [-source-check=false]
  plugproxy fetch [-app-config plugproxy.config.json] [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-per-host-workers 4] [-source-cooldown 15m]
  plugproxy check [-app-config plugproxy.config.json] [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-per-host-workers 4] [-workers 32] [-protocol http] [-target URL] [-target-fallbacks URLS] [-body-contains TEXT] [-timeout 8s] [-max-checks 0] [-check-profile full] [-source-fair=false] [-tail-biased=false] [-connect-timeout 5s]
  plugproxy list [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-exclude-dead=false]
  plugproxy get [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-strategy fastest] [-protocol http] [-healthy=false] [-exclude-dead=true]
  plugproxy stats [-cache .plugproxy.cache.json] [-fetch=false]
  plugproxy watch [-api http://127.0.0.1:8899] [-interval 1s]
  plugproxy run [-app-config plugproxy.config.json] [-config plugproxy.sources.json] [-cache .plugproxy.cache.json] [-source-workers 32] [-per-host-workers 4] [-source-cooldown 15m] [-addr 127.0.0.1:8899] [-skip-check=true] [-refresh=true] [-refresh-interval 5m] [-refresh-min-interval 30s] [-refresh-max-interval 30m] [-refresh-jitter 10s] [-min-healthy 1] [-min-healthy-ratio 0] [-unchecked-threshold 100] [-max-checks 0] [-check-profile smart] [-source-fair=true] [-tail-biased=true] [-shutdown-timeout 10s] [-log-level info] [-log-format text]
  plugproxy discover repo owner/name
  plugproxy discover url URL
  plugproxy discover validate FILE [-write-sources plugproxy.sources.candidates.json]
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

func runConfigCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: plugproxy config init|validate|print")
	}
	switch args[0] {
	case "init":
		fs := flag.NewFlagSet("config init", flag.ExitOnError)
		appConfigPath := fs.String("app-config", config.DefaultAppPath, "app config path")
		force := fs.Bool("force", false, "overwrite existing app config")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"app-config": false, "force": true}))
		if !*force {
			if _, err := os.Stat(*appConfigPath); err == nil {
				return fmt.Errorf("%s already exists; use -force to overwrite", *appConfigPath)
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		if err := config.SaveApp(*appConfigPath, config.DefaultAppConfig()); err != nil {
			return err
		}
		writeJSON(map[string]any{"app_config": *appConfigPath, "created": true})
	case "validate":
		fs := flag.NewFlagSet("config validate", flag.ExitOnError)
		appConfigPath := fs.String("app-config", config.DefaultAppPath, "app config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"app-config": false}))
		cfg, err := config.LoadApp(*appConfigPath)
		if err != nil {
			return err
		}
		if err := config.ValidateApp(cfg); err != nil {
			return err
		}
		writeJSON(map[string]any{"app_config": *appConfigPath, "ok": true})
	case "print":
		fs := flag.NewFlagSet("config print", flag.ExitOnError)
		appConfigPath := fs.String("app-config", config.DefaultAppPath, "app config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"app-config": false}))
		cfg, err := config.LoadApp(*appConfigPath)
		if err != nil {
			return err
		}
		writeJSON(cfg.WithDefaults())
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
	return nil
}

func runCacheCommand(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: plugproxy cache stats|compact|repair")
	}
	switch args[0] {
	case "stats":
		fs := flag.NewFlagSet("cache stats", flag.ExitOnError)
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"cache": false}))
		stats, err := cache.FileStats(*cachePath)
		if err != nil {
			return err
		}
		writeJSON(stats)
	case "compact":
		fs := flag.NewFlagSet("cache compact", flag.ExitOnError)
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		maxEntries := fs.Int("max-entries", 0, "maximum proxies to keep; 0 keeps all")
		dropDeadAfter := fs.Duration("drop-dead-after", 0, "drop dead proxies older than this duration")
		dropStaleAfter := fs.Duration("drop-stale-after", 0, "drop proxies not seen within this duration")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{
			"cache": false, "max-entries": false, "drop-dead-after": false, "drop-stale-after": false,
		}))
		report, err := cache.Compact(*cachePath, cache.CompactOptions{
			MaxEntries:     *maxEntries,
			DropDeadAfter:  *dropDeadAfter,
			DropStaleAfter: *dropStaleAfter,
		})
		if err != nil {
			return err
		}
		writeJSON(report)
	case "repair":
		fs := flag.NewFlagSet("cache repair", flag.ExitOnError)
		cachePath := fs.String("cache", cache.DefaultPath, "proxy cache path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"cache": false}))
		writeJSON(cache.Repair(*cachePath))
	default:
		return fmt.Errorf("unknown cache command %q", args[0])
	}
	return nil
}

func runWatch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	apiURL := fs.String("api", client.DefaultBaseURL, "plugproxy API base URL")
	interval := fs.Duration("interval", time.Second, "poll interval")
	count := fs.Int("count", 0, "number of samples; 0 means until interrupted")
	_ = fs.Parse(reorderFlagArgs(args, map[string]bool{"api": false, "interval": false, "count": false}))
	if *interval <= 0 {
		*interval = time.Second
	}
	c := client.New(*apiURL)
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for sample := 0; ; sample++ {
		if *count > 0 && sample >= *count {
			return nil
		}
		metrics, err := c.Metrics(ctx)
		if err != nil {
			return err
		}
		fmt.Println(formatWatchLine(metrics))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func formatWatchLine(metrics map[string]any) string {
	poolValue, _ := metrics["pool"].(map[string]any)
	checkValue, _ := metrics["check"].(map[string]any)
	refreshValue, _ := metrics["refresh"].(map[string]any)
	return fmt.Sprintf(
		"pool total=%v healthy=%v degraded=%v dead=%v unchecked=%v check scheduled=%v failed=%v refresh status=%v phase=%v reason=%v",
		poolValue["total"],
		poolValue["healthy"],
		poolValue["degraded"],
		poolValue["dead"],
		poolValue["unchecked"],
		checkValue["scheduled"],
		checkValue["failed"],
		refreshValue["status"],
		refreshValue["phase"],
		refreshValue["last_reason"],
	)
}

type sourceListItem struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	URL          string `json:"url"`
	ProtocolHint string `json:"protocol_hint,omitempty"`
	Enabled      bool   `json:"enabled"`
}

func runSourcesCommand(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: plugproxy sources list|validate|test|add|remove|enable|disable|export")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("sources list", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"config": false}))
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		items := make([]sourceListItem, 0, len(cfg.Sources))
		for _, item := range cfg.Sources {
			items = append(items, sourceListItem{
				Name:         item.Name,
				Type:         item.Type,
				URL:          item.URL,
				ProtocolHint: item.ProtocolHint,
				Enabled:      item.Enabled == nil || *item.Enabled,
			})
		}
		writeJSON(map[string]any{"config": *configPath, "sources": items})
	case "validate":
		fs := flag.NewFlagSet("sources validate", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"config": false}))
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		sources, err := config.BuildSources(cfg)
		if err != nil {
			return err
		}
		writeJSON(map[string]any{"config": *configPath, "ok": true, "total": len(cfg.Sources), "enabled": len(sources)})
	case "test":
		fs := flag.NewFlagSet("sources test", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		workers := fs.Int("source-workers", 32, "number of concurrent source fetches")
		perHostWorkers := fs.Int("per-host-workers", 4, "maximum concurrent source fetches per host")
		timeout := fs.Duration("timeout", 0, "reserved for source-specific timeouts")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{
			"config": false, "source-workers": false, "per-host-workers": false, "timeout": false,
		}))
		_ = timeout
		sources, err := config.LoadSources(*configPath)
		if err != nil {
			return err
		}
		report := app.NewWithSources(slog.Default(), sources).FetchWithOptions(ctx, app.FetchOptions{
			Workers:        *workers,
			PerHostWorkers: *perHostWorkers,
			CacheWrite:     false,
			CacheFallback:  false,
		})
		writeJSON(report)
	case "enable", "disable":
		enabled := args[0] == "enable"
		fs := flag.NewFlagSet("sources "+args[0], flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"config": false}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy sources %s NAME", args[0])
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		if !setSourceEnabled(&cfg, fs.Arg(0), enabled) {
			return fmt.Errorf("source %q not found", fs.Arg(0))
		}
		if err := config.Save(*configPath, cfg); err != nil {
			return err
		}
		writeJSON(map[string]any{"config": *configPath, "source": fs.Arg(0), "enabled": enabled})
	case "add":
		fs := flag.NewFlagSet("sources add", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		name := fs.String("name", "", "source name")
		sourceType := fs.String("type", "raw_text_url", "source type")
		sourceURL := fs.String("url", "", "source URL")
		protocolHint := fs.String("protocol-hint", "", "protocol hint")
		enabled := fs.Bool("enabled", false, "enable source immediately")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{
			"config": false, "name": false, "type": false, "url": false, "protocol-hint": false, "enabled": true,
		}))
		if *sourceURL == "" {
			return fmt.Errorf("sources add requires -url")
		}
		if *name == "" {
			*name = candidateSourceName(discover.CandidateSource{URL: *sourceURL}, 1)
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		if sourceIndex(cfg, *name) >= 0 {
			return fmt.Errorf("source %q already exists", *name)
		}
		item := config.SourceConfig{
			Name:         *name,
			Type:         *sourceType,
			URL:          *sourceURL,
			ProtocolHint: *protocolHint,
			Enabled:      enabled,
		}
		cfg.Sources = append(cfg.Sources, item)
		if _, err := config.BuildSources(config.Config{Sources: []config.SourceConfig{item}}); err != nil {
			return err
		}
		if err := config.Save(*configPath, cfg); err != nil {
			return err
		}
		writeJSON(map[string]any{"config": *configPath, "source": item})
	case "remove":
		fs := flag.NewFlagSet("sources remove", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"config": false}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy sources remove NAME")
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		index := sourceIndex(cfg, fs.Arg(0))
		if index < 0 {
			return fmt.Errorf("source %q not found", fs.Arg(0))
		}
		cfg.Sources = append(cfg.Sources[:index], cfg.Sources[index+1:]...)
		if err := config.Save(*configPath, cfg); err != nil {
			return err
		}
		writeJSON(map[string]any{"config": *configPath, "removed": fs.Arg(0)})
	case "export":
		fs := flag.NewFlagSet("sources export", flag.ExitOnError)
		configPath := fs.String("config", config.DefaultPath, "source config path")
		outputPath := fs.String("output", "", "output source config path")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{"config": false, "output": false}))
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		if *outputPath == "" {
			writeJSON(cfg)
			return nil
		}
		if err := config.Save(*outputPath, cfg); err != nil {
			return err
		}
		writeJSON(map[string]any{"config": *configPath, "output": *outputPath, "exported": len(cfg.Sources)})
	default:
		return fmt.Errorf("unknown sources command %q", args[0])
	}
	return nil
}

func setSourceEnabled(cfg *config.Config, name string, enabled bool) bool {
	index := sourceIndex(*cfg, name)
	if index < 0 {
		return false
	}
	cfg.Sources[index].Enabled = &enabled
	return true
}

func sourceIndex(cfg config.Config, name string) int {
	for index, item := range cfg.Sources {
		if item.Name == name {
			return index
		}
	}
	return -1
}

func loadAppDefaults(args []string) (config.AppConfig, string, bool, error) {
	path, explicit := flagStringValue(args, "app-config", config.DefaultAppPath)
	if explicit || fileExists(path) {
		cfg, err := config.LoadApp(path)
		return cfg, path, true, err
	}
	return config.DefaultAppConfig(), path, false, nil
}

func flagStringValue(args []string, name string, fallback string) (string, bool) {
	prefix := "-" + name + "="
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-"+name && i+1 < len(args) {
			return args[i+1], true
		}
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix), true
		}
	}
	return fallback, false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func newLogger(level string, format string) *slog.Logger {
	options := &slog.HandlerOptions{Level: slogLevel(level)}
	if strings.EqualFold(format, "json") {
		return slog.New(slog.NewJSONHandler(os.Stderr, options))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, options))
}

func slogLevel(value string) slog.Level {
	switch strings.ToLower(value) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func schedulerProfile(value string) scheduler.CheckProfile {
	switch strings.ToLower(value) {
	case string(scheduler.ProfileSmart):
		return scheduler.ProfileSmart
	default:
		return scheduler.ProfileFull
	}
}

func targetURLs(primary string, targetHTTP string, targetHTTPS string, fallbacks string, configured ...[]string) []string {
	values := make([]string, 0, 4)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range values {
			if existing == value {
				return
			}
		}
		values = append(values, value)
	}
	add(primary)
	add(targetHTTP)
	add(targetHTTPS)
	for _, item := range strings.Split(fallbacks, ",") {
		add(item)
	}
	for _, items := range configured {
		for _, item := range items {
			add(item)
		}
	}
	return values
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	wasSet := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			wasSet = true
		}
	})
	return wasSet
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
		perHostWorkers := fs.Int("per-host-workers", 4, "maximum concurrent validations per host")
		writeSources := fs.String("write-sources", "", "write validated source candidates to a plugproxy source config")
		_ = fs.Parse(reorderFlagArgs(args[1:], map[string]bool{
			"timeout": false, "workers": false, "per-host-workers": false, "write-sources": false,
		}))
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: plugproxy discover validate FILE")
		}
		report, err := readDiscoveryInput(fs.Arg(0))
		if err != nil {
			return err
		}
		report.Source = "validate"
		report.Generated = time.Now()
		report.Candidates = discover.NewValidatorWithOptions(discover.ValidatorOptions{
			Timeout:        *timeout,
			Workers:        *workers,
			PerHostWorkers: *perHostWorkers,
		}).Validate(ctx, report.Candidates)
		if *writeSources != "" {
			if err := config.Save(*writeSources, sourcesConfigFromCandidates(report.Candidates)); err != nil {
				return err
			}
		}
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
  plugproxy discover validate FILE [-write-sources plugproxy.sources.candidates.json]
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

func sourcesConfigFromCandidates(candidates []discover.CandidateSource) config.Config {
	enabled := false
	cfg := config.Config{Sources: make([]config.SourceConfig, 0)}
	for _, candidate := range candidates {
		if candidate.Status != discover.StatusValid || candidate.AdapterRequired || candidate.URL == "" {
			continue
		}
		sourceType, ok := sourceTypeForCandidate(candidate)
		if !ok {
			continue
		}
		protocolHint := candidate.ProtocolHint
		if protocolHint == "" && candidate.Recipe != nil {
			protocolHint = candidate.Recipe.ProtocolHint
		}
		item := config.SourceConfig{
			Name:         candidateSourceName(candidate, len(cfg.Sources)+1),
			Type:         sourceType,
			URL:          candidate.URL,
			ProtocolHint: protocolHint,
			Enabled:      &enabled,
		}
		if sourceType == "json_url" || sourceType == "api_url" {
			item.JSON = &source.JSONConfig{}
		}
		cfg.Sources = append(cfg.Sources, item)
	}
	return cfg
}

func sourceTypeForCandidate(candidate discover.CandidateSource) (string, bool) {
	kind := candidate.SourceKind
	if candidate.Recipe != nil && candidate.Recipe.Kind != "" {
		kind = candidate.Recipe.Kind
	}
	switch kind {
	case discover.KindRawText:
		return "raw_text_url", true
	case discover.KindHTMLText:
		return "html_text_url", true
	case discover.KindJSON:
		return "json_url", true
	case discover.KindAPI:
		return "api_url", true
	default:
		return "", false
	}
}

func candidateSourceName(candidate discover.CandidateSource, index int) string {
	if candidate.Name != "" {
		return slug(candidate.Name)
	}
	parsed, err := url.Parse(candidate.URL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Sprintf("candidate-%d", index)
	}
	base := strings.TrimSuffix(path.Base(parsed.Path), path.Ext(parsed.Path))
	if base == "." || base == "/" || base == "" {
		base = "source"
	}
	return slug(parsed.Hostname() + "-" + base)
}

func slug(value string) string {
	value = strings.ToLower(value)
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "candidate"
	}
	return result
}
