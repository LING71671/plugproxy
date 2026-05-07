package plugproxy

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/LING71671/plugproxy/internal/app"
	"github.com/LING71671/plugproxy/internal/cache"
	"github.com/LING71671/plugproxy/internal/checker"
	internalconfig "github.com/LING71671/plugproxy/internal/config"
	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/scheduler"
	"github.com/LING71671/plugproxy/pkg/client"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Config struct {
	Addr                  string
	ConfigPath            string
	CachePath             string
	CacheFallback         bool
	SourceWorkers         int
	CheckWorkers          int
	TargetURL             string
	CheckTimeout          time.Duration
	ConnectTimeout        time.Duration
	TLSHandshakeTimeout   time.Duration
	ResponseHeaderTimeout time.Duration
	IdleConnTimeout       time.Duration
	MaxIdleConns          int
	MaxIdleConnsPerHost   int
	SkipCheck             bool
	Refresh               bool
	RefreshInterval       time.Duration
	Logger                *slog.Logger
}

type Service struct {
	config   Config
	app      *app.App
	server   *http.Server
	listener net.Listener
	cancel   context.CancelFunc
	url      string
	client   *client.Client
}

func New(config Config) *Service {
	return &Service{config: withDefaults(config)}
}

func (s *Service) Start(ctx context.Context) error {
	if s.server != nil {
		return fmt.Errorf("plugproxy service already started")
	}

	sources, err := internalconfig.LoadSources(s.config.ConfigPath)
	if err != nil {
		return err
	}
	s.app = app.NewWithSources(s.config.Logger, sources)
	serviceCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	refreshOptions := s.refreshOptions()
	s.app.FetchWithOptions(ctx, refreshOptions.Fetch)
	if !s.config.SkipCheck {
		s.app.CheckWithOptions(ctx, refreshOptions.Check)
	}
	if s.config.Refresh {
		s.app.StartAutoRefresh(serviceCtx, s.config.RefreshInterval, refreshOptions)
	}

	listener, err := net.Listen("tcp", s.config.Addr)
	if err != nil {
		cancel()
		s.cancel = nil
		return err
	}
	s.listener = listener
	s.url = "http://" + listener.Addr().String()
	s.client = client.New(s.url)
	s.server = &http.Server{Handler: s.app.Handler(refreshOptions)}
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed && s.config.Logger != nil {
			s.config.Logger.Error("embedded plugproxy server stopped", "error", err)
		}
	}()
	return nil
}

func (s *Service) Close(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	err := s.server.Shutdown(ctx)
	s.server = nil
	s.listener = nil
	s.cancel = nil
	s.client = nil
	s.url = ""
	return err
}

func (s *Service) URL() string {
	return s.url
}

func (s *Service) Client() *client.Client {
	return s.client
}

func (s *Service) GetProxy(ctx context.Context, options client.GetProxyOptions) (model.Proxy, error) {
	if s.client == nil {
		return model.Proxy{}, fmt.Errorf("plugproxy service is not started")
	}
	return s.client.GetProxy(ctx, options)
}

func (s *Service) ListProxies(ctx context.Context, options client.ListOptions) ([]model.Proxy, error) {
	if s.client == nil {
		return nil, fmt.Errorf("plugproxy service is not started")
	}
	return s.client.ListProxies(ctx, options)
}

func (s *Service) Refresh(ctx context.Context) (map[string]any, error) {
	if s.client == nil {
		return nil, fmt.Errorf("plugproxy service is not started")
	}
	return s.client.TriggerRefresh(ctx)
}

func (s *Service) CancelRefresh(ctx context.Context) (map[string]any, error) {
	if s.client == nil {
		return nil, fmt.Errorf("plugproxy service is not started")
	}
	return s.client.CancelRefresh(ctx)
}

func (s *Service) refreshOptions() app.RefreshOptions {
	return app.RefreshOptions{
		Fetch: app.FetchOptions{
			Workers:       s.config.SourceWorkers,
			CachePath:     s.config.CachePath,
			CacheFallback: s.config.CacheFallback,
			CacheWrite:    true,
		},
		Check: app.CheckOptions{
			Workers:         s.config.CheckWorkers,
			TargetURL:       s.config.TargetURL,
			Timeout:         s.config.CheckTimeout,
			CachePath:       s.config.CachePath,
			CacheWrite:      true,
			Filter:          pool.Filter{},
			Profile:         scheduler.ProfileSmart,
			SkipUnsupported: true,
			ProtocolFair:    true,
			SourceFair:      true,
			TailBiased:      true,
			Transport: checker.TransportOptions{
				ConnectTimeout:        s.config.ConnectTimeout,
				TLSHandshakeTimeout:   s.config.TLSHandshakeTimeout,
				ResponseHeaderTimeout: s.config.ResponseHeaderTimeout,
				IdleConnTimeout:       s.config.IdleConnTimeout,
				MaxIdleConns:          s.config.MaxIdleConns,
				MaxIdleConnsPerHost:   s.config.MaxIdleConnsPerHost,
			},
		},
	}
}

func withDefaults(config Config) Config {
	if config.Addr == "" {
		config.Addr = "127.0.0.1:0"
	}
	if config.ConfigPath == "" {
		config.ConfigPath = internalconfig.DefaultPath
	}
	if config.CachePath == "" {
		config.CachePath = cache.DefaultPath
	}
	if config.SourceWorkers <= 0 {
		config.SourceWorkers = 32
	}
	if config.CheckWorkers <= 0 {
		config.CheckWorkers = 32
	}
	if config.TargetURL == "" {
		config.TargetURL = "https://httpbin.org/ip"
	}
	if config.CheckTimeout <= 0 {
		config.CheckTimeout = 8 * time.Second
	}
	if config.RefreshInterval <= 0 {
		config.RefreshInterval = 5 * time.Minute
	}
	return config
}
