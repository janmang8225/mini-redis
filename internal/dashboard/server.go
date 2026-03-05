package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/janmang8225/mini-redis/internal/metrics"
	"github.com/janmang8225/mini-redis/internal/pubsub"
	"github.com/janmang8225/mini-redis/internal/store"
)

//go:embed index.html
var staticFiles embed.FS

// Server is the dashboard HTTP server.
type Server struct {
	addr   string
	store  *store.Store
	broker *pubsub.Broker
	httpSrv *http.Server
}

func New(addr string, st *store.Store, broker *pubsub.Broker) *Server {
	return &Server{
		addr:   addr,
		store:  st,
		broker: broker,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// serve the dashboard UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFiles.ReadFile("index.html")
		if err != nil {
			http.Error(w, "dashboard not found", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	// REST API — all return JSON
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/keys", s.handleKeys)
	mux.HandleFunc("/api/key", s.handleKey)

	s.httpSrv = &http.Server{
		Addr:         s.addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("dashboard listening", "addr", s.addr)
	return s.httpSrv.ListenAndServe()
}

func (s *Server) Stop() {
	if s.httpSrv != nil {
		s.httpSrv.Close()
	}
}

// GET /api/stats — server stats + metrics
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	snap := metrics.Global.Snapshot()

	resp := map[string]any{
		"uptime_seconds": snap.UptimeSeconds,
		"uptime_human":   formatUptime(snap.UptimeSeconds),
		"cmd_total":      snap.CmdTotal,
		"cmd_per_sec":    snap.CmdPerSec,
		"conn_total":     snap.ConnTotal,
		"conn_current":   snap.ConnCurrent,
		"total_keys":     s.store.DBSize(),
		"pubsub_clients": s.broker.NumSubscribers(),
	}

	writeJSON(w, resp)
}

// GET /api/keys — list all keys with type and TTL
func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	keys := s.store.Keys()

	type keyInfo struct {
		Key  string `json:"key"`
		Type string `json:"type"`
		TTL  int64  `json:"ttl"` // -1 = no expiry, -2 = missing, >=0 seconds
	}

	result := make([]keyInfo, 0, len(keys))
	for _, k := range keys {
		ttlDur := s.store.TTL(k)
		var ttlSec int64
		switch ttlDur {
		case -2:
			ttlSec = -2
		case -1:
			ttlSec = -1
		default:
			ttlSec = int64(ttlDur.Seconds())
		}

		result = append(result, keyInfo{
			Key:  k,
			Type: s.store.GetType(k),
			TTL:  ttlSec,
		})
	}

	writeJSON(w, map[string]any{
		"count": len(result),
		"keys":  result,
	})
}

// GET /api/key?k=<key> — inspect a single key's value
func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("k")
	if key == "" {
		http.Error(w, "missing ?k= param", 400)
		return
	}

	keyType := s.store.GetType(key)
	ttlDur := s.store.TTL(key)
	var ttlSec int64
	switch ttlDur {
	case -2:
		http.Error(w, "key not found", 404)
		return
	case -1:
		ttlSec = -1
	default:
		ttlSec = int64(ttlDur.Seconds())
	}

	var value any
	switch keyType {
	case "string":
		v, _ := s.store.GetString(key)
		value = v
	case "list":
		v, _ := s.store.LRange(key, 0, -1)
		value = v
	case "hash":
		v, _ := s.store.HGetAll(key)
		value = v
	case "set":
		v, _ := s.store.SMembers(key)
		value = v
	}

	writeJSON(w, map[string]any{
		"key":   key,
		"type":  keyType,
		"ttl":   ttlSec,
		"value": value,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("dashboard json encode failed", "err", err)
	}
}

func formatUptime(secs int64) string {
	d := time.Duration(secs) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}