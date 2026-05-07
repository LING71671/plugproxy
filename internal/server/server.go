package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/internal/web"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Server struct {
	pool          pool.Pool
	log           *slog.Logger
	sourceReport  func() any
	metrics       func() any
	refresh       func(context.Context) any
	refreshState  func() any
	refreshCancel func() any
}

func New(proxyPool pool.Pool, log *slog.Logger) Server {
	if log == nil {
		log = slog.Default()
	}

	return Server{pool: proxyPool, log: log}
}

func (s Server) WithSourceReport(report func() any) Server {
	s.sourceReport = report
	return s
}

func (s Server) WithMetrics(metrics func() any) Server {
	s.metrics = metrics
	return s
}

func (s Server) WithRefresh(refresh func(context.Context) any, state func() any, cancel ...func() any) Server {
	s.refresh = refresh
	s.refreshState = state
	if len(cancel) > 0 {
		s.refreshCancel = cancel[0]
	}
	return s
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /metrics.json", s.getMetrics)
	mux.Handle("GET /ui", http.RedirectHandler("/ui/", http.StatusMovedPermanently))
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", web.Handler()))
	mux.HandleFunc("GET /sources", s.sources)
	mux.HandleFunc("GET /refresh", s.getRefresh)
	mux.HandleFunc("POST /refresh", s.postRefresh)
	mux.HandleFunc("POST /refresh/cancel", s.cancelRefresh)
	mux.HandleFunc("GET /stats", s.stats)
	mux.HandleFunc("GET /proxies", s.listProxies)
	mux.HandleFunc("GET /proxy", s.getProxy)
	return mux
}

func (s Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/ui", http.StatusFound)
}

func (s Server) cancelRefresh(w http.ResponseWriter, _ *http.Request) {
	if s.refreshCancel == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "refresh cancel is not configured"})
		return
	}
	writeJSON(w, http.StatusAccepted, s.refreshCancel())
}

func (s Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s Server) listProxies(w http.ResponseWriter, r *http.Request) {
	items := s.pool.List(parseFilter(r))
	offset := parseNonNegativeInt(r, "offset")
	limit := parseNonNegativeInt(r, "limit")
	if offset > len(items) {
		offset = len(items)
	}
	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	writeJSON(w, http.StatusOK, items)
}

func (s Server) sources(w http.ResponseWriter, _ *http.Request) {
	if s.sourceReport == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sources": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, s.sourceReport())
}

func (s Server) getMetrics(w http.ResponseWriter, _ *http.Request) {
	if s.metrics == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "metrics_unconfigured"})
		return
	}
	writeJSON(w, http.StatusOK, s.metrics())
}

func (s Server) getRefresh(w http.ResponseWriter, _ *http.Request) {
	if s.refreshState == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "idle"})
		return
	}
	writeJSON(w, http.StatusOK, s.refreshState())
}

func (s Server) postRefresh(w http.ResponseWriter, r *http.Request) {
	if s.refresh == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "refresh is not configured"})
		return
	}
	writeJSON(w, http.StatusAccepted, s.refresh(context.WithoutCancel(r.Context())))
}

func (s Server) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, model.NewProxyStats(s.pool.List(pool.Filter{})))
}

func (s Server) getProxy(w http.ResponseWriter, r *http.Request) {
	strategy := pool.Strategy(r.URL.Query().Get("strategy"))
	if strategy == "" {
		strategy = pool.StrategyAny
	}

	proxy, ok := s.pool.Get(strategy, parseFilter(r))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no proxy available"})
		return
	}

	writeJSON(w, http.StatusOK, proxy)
}

func parseFilter(r *http.Request) pool.Filter {
	filter := pool.Filter{
		Protocol: model.Protocol(r.URL.Query().Get("protocol")),
		Healthy:  r.URL.Query().Get("healthy") == "true",
		Status:   model.HealthStatus(r.URL.Query().Get("status")),
		Source:   r.URL.Query().Get("source"),
	}
	if filter.Healthy && filter.Status == "" {
		filter.Status = model.HealthHealthy
	}
	return filter
}

func parseNonNegativeInt(r *http.Request, name string) int {
	value := r.URL.Query().Get(name)
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
