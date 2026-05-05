package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/LING71671/plugproxy/internal/pool"
	"github.com/LING71671/plugproxy/pkg/model"
)

type Server struct {
	pool         pool.Pool
	log          *slog.Logger
	sourceReport func() any
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

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /sources", s.sources)
	mux.HandleFunc("GET /proxies", s.listProxies)
	mux.HandleFunc("GET /proxy", s.getProxy)
	return mux
}

func (s Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s Server) listProxies(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.pool.List(parseFilter(r)))
}

func (s Server) sources(w http.ResponseWriter, _ *http.Request) {
	if s.sourceReport == nil {
		writeJSON(w, http.StatusOK, map[string]any{"sources": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, s.sourceReport())
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
	return pool.Filter{
		Protocol: model.Protocol(r.URL.Query().Get("protocol")),
		Healthy:  r.URL.Query().Get("healthy") == "true",
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
