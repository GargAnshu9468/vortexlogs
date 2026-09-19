package server

import (
	"bufio"
	"bytes"
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

	server := &http.Server{
		Addr:         s.addr,
		Handler:      s.corsMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("[VortexLogs] 🌌 Quantum Log Studio & HTTP Ingest running on http://%s\n", s.addr)
	return server.ListenAndServe()
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
	q := r.URL.Query()

	req := engine.QueryRequest{
		SubstringFilter: q.Get("search"),
		Limit:           100,
	}

	if lvl := q.Get("level"); lvl != "" {
		req.Level = storage.ParseLevel(lvl)
	}

	if srv := q.Get("service"); srv != "" {
		if req.LabelSelectors == nil {
			req.LabelSelectors = make(map[string]string)
		}
		req.LabelSelectors["service"] = srv
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

	for entry := range logCh {
		if err := conn.WriteJSON(entry); err != nil {
			break
		}
	}
}
