package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
	"github.com/GargAnshu9468/vortexlogs/internal/storage"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// HTTPServer provides HTTP REST endpoints, WebSocket live tailing, and Web Studio serving.
type HTTPServer struct {
	addr      string
	eng       *engine.Engine
	staticFS  http.FileSystem
	clientSeq uint64
	server    *http.Server
}

// NewHTTPServer initializes a new HTTP ingestion and Web Studio server.
func NewHTTPServer(addr string, eng *engine.Engine, staticFS http.FileSystem) *HTTPServer {
	return &HTTPServer{
		addr:     addr,
		eng:      eng,
		staticFS: staticFS,
	}
}

// ListenAndServe starts the HTTP server.
func (s *HTTPServer) ListenAndServe() error {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","engine":"vortexlogs"}`))
	})
	mux.HandleFunc("/api/v1/ingest", s.handleIngest)
	mux.HandleFunc("/api/v1/query", s.handleQuery)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/tail", s.handleTailWebSocket)

	// Favicon fallback
	faviconSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"><defs><linearGradient id="g" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse"><stop offset="0%" stop-color="#00f0ff"/><stop offset="50%" stop-color="#8b5cf6"/><stop offset="100%" stop-color="#ec4899"/></linearGradient><linearGradient id="b" x1="40" y1="0" x2="0" y2="40" gradientUnits="userSpaceOnUse"><stop offset="0%" stop-color="#00f0ff"/><stop offset="100%" stop-color="#3b82f6"/></linearGradient></defs><path d="M20 3C30.4934 3 39 10.6112 39 20C39 24.2 37.4 28.1 34.6 31L29.8 26.2C31.2 24.4 32 22.3 32 20C32 13.9249 26.6274 9 20 9C15.8 9 12.1 11.2 10.1 14.5L4.8 11.2C8 6.2 13.6 3 20 3Z" fill="url(#g)"/><path d="M20 37C9.5066 37 1 29.3888 1 20C1 15.8 2.6 11.9 5.4 9L10.2 13.8C8.8 15.6 8 17.7 8 20C8 26.0751 13.3726 31 20 31C24.2 31 27.9 28.8 29.9 25.5L35.2 28.8C32 33.8 26.4 37 20 37Z" fill="url(#g)"/><rect x="18" y="13" width="4" height="14" rx="2" fill="url(#b)"/><circle cx="20" cy="20" r="2.5" fill="#ffffff"/></svg>`)
	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Write(faviconSVG)
	})

	// Embedded Quantum Log Studio GUI
	if s.staticFS != nil {
		fileServer := http.FileServer(s.staticFS)
		mux.Handle("/", fileServer)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("VortexLogs API running. Studio UI not embedded."))
		})
	}

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("[VortexLogs] 🌌 Quantum Log Studio & HTTP Ingest running on http://%s\n", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *HTTPServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleIngest handles single-log, JSON array, or ndjson stream ingestion.
func (s *HTTPServer) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 32*1024*1024))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		http.Error(w, "Empty body", http.StatusBadRequest)
		return
	}

	var entries []*storage.Entry

	if trimmed[0] == '[' {
		// JSON Array of objects
		if err := json.Unmarshal(trimmed, &entries); err != nil {
			http.Error(w, "Invalid JSON array: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else if trimmed[0] == '{' {
		// Single JSON log object OR line-delimited JSON (ndjson)
		if strings.Contains(string(trimmed), "\n") {
			scanner := bufio.NewScanner(bytes.NewReader(trimmed))
			for scanner.Scan() {
				line := bytes.TrimSpace(scanner.Bytes())
				if len(line) == 0 {
					continue
				}
				var e storage.Entry
				if err := json.Unmarshal(line, &e); err == nil {
					entries = append(entries, &e)
				}
			}
		} else {
			var e storage.Entry
			if err := json.Unmarshal(trimmed, &e); err != nil {
				http.Error(w, "Invalid JSON object: "+err.Error(), http.StatusBadRequest)
				return
			}
			entries = append(entries, &e)
		}
	} else {
		// Plain text log line
		e := storage.NewEntry(storage.LevelInfo, "default", string(trimmed), nil)
		entries = append(entries, e)
	}

	if err := s.eng.IngestBatch(entries); err != nil {
		http.Error(w, "Ingestion failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "ok",
		"ingested": len(entries),
	})
}

// handleQuery handles time-range, label-filtered, and regex full-text queries.
func (s *HTTPServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	req := engine.QueryRequest{
		Limit: 100,
	}

	// 1. If POST with JSON body, unmarshal into req
	if r.Method == http.MethodPost && r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err == nil && len(bytes.TrimSpace(body)) > 0 {
			var jsonReq struct {
				StartTime       int64             `json:"start_time"`
				EndTime         int64             `json:"end_time"`
				Level           string            `json:"level"`
				Labels          map[string]string `json:"labels"`
				Search          string            `json:"search"`
				Limit           int               `json:"limit"`
				HistogramBuckets int              `json:"histogram_buckets"`
			}
			if err := json.Unmarshal(body, &jsonReq); err == nil {
				req.StartTime = jsonReq.StartTime
				req.EndTime = jsonReq.EndTime
				if jsonReq.Level != "" {
					req.Level = storage.ParseLevel(jsonReq.Level)
				}
				req.LabelSelectors = jsonReq.Labels
				req.SubstringFilter = jsonReq.Search
				if jsonReq.Limit > 0 {
					req.Limit = jsonReq.Limit
				}
				req.HistogramBuckets = jsonReq.HistogramBuckets
			}
		}
	}

	// 2. Query URL params override or augment
	q := r.URL.Query()
	if search := q.Get("search"); search != "" {
		req.SubstringFilter = search
	}
	if lvl := q.Get("level"); lvl != "" {
		req.Level = storage.ParseLevel(lvl)
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			req.Limit = l
		}
	}
	if startStr := q.Get("start"); startStr != "" {
		if ts, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			req.StartTime = ts
		}
	}
	if endStr := q.Get("end"); endStr != "" {
		if ts, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			req.EndTime = ts
		}
	}

	// Support any arbitrary query param as label filter (e.g. ?service=auth&env=prod)
	for key, values := range q {
		if key != "search" && key != "limit" && key != "level" && key != "start" && key != "end" && len(values) > 0 {
			if req.LabelSelectors == nil {
				req.LabelSelectors = make(map[string]string)
			}
			req.LabelSelectors[key] = values[0]
		}
	}

	res, err := s.eng.Query(req)
	if err != nil {
		http.Error(w, "Query error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// handleStats returns engine telemetry and compression metrics.
func (s *HTTPServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := s.eng.Stats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleTailWebSocket streams live incoming logs directly to Web Studio clients.
func (s *HTTPServer) handleTailWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	clientID := fmt.Sprintf("ws-%d", atomic.AddUint64(&s.clientSeq, 1))
	filter := r.URL.Query().Get("filter")

	logCh := s.eng.Subscribe(clientID, filter)
	defer s.eng.Unsubscribe(clientID)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case entry, ok := <-logCh:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(entry); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
