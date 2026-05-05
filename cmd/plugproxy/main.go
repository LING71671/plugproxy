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
	"github.com/LING71671/plugproxy/internal/discover"
	"github.com/LING71671/plugproxy/internal/pool"
)

const version = "0.1.0-dev"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	application := app.New(log)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx := context.Background()

	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "fetch":
		count := application.Fetch(ctx)
		fmt.Printf("fetched %d proxies\n", count)
	case "check":
		fs := flag.NewFlagSet("check", flag.ExitOnError)
		workers := fs.Int("workers", 32, "number of concurrent proxy checks")
		target := fs.String("target", "https://httpbin.org/ip", "target URL used to check proxies")
		timeout := fs.Duration("timeout", 8*time.Second, "per-proxy check timeout")
		_ = fs.Parse(os.Args[2:])
		application.Fetch(ctx)
		healthy := application.Check(ctx, *workers, *target, *timeout)
		fmt.Printf("healthy %d proxies\n", healthy)
	case "list":
		application.Fetch(ctx)
		writeJSON(application.Pool().List(pool.Filter{}))
	case "get":
		application.Fetch(ctx)
		proxy, ok := application.Pool().Get(pool.StrategyAny, pool.Filter{})
		if !ok {
			fmt.Fprintln(os.Stderr, "no proxy available")
			os.Exit(1)
		}
		writeJSON(proxy)
	case "run":
		fs := flag.NewFlagSet("run", flag.ExitOnError)
		addr := fs.String("addr", "127.0.0.1:8899", "HTTP API listen address")
		workers := fs.Int("workers", 32, "number of concurrent proxy checks")
		target := fs.String("target", "https://httpbin.org/ip", "target URL used to check proxies")
		timeout := fs.Duration("timeout", 8*time.Second, "per-proxy check timeout")
		skipCheck := fs.Bool("skip-check", true, "skip proxy checking on startup")
		_ = fs.Parse(os.Args[2:])

		application.Fetch(ctx)
		if !*skipCheck {
			application.Check(ctx, *workers, *target, *timeout)
		}
		if err := application.Serve(*addr); err != nil {
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
  plugproxy fetch
  plugproxy check [-workers 32] [-target URL] [-timeout 8s]
  plugproxy list
  plugproxy get
  plugproxy run [-addr 127.0.0.1:8899] [-skip-check=true]
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
