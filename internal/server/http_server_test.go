package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
)

func setupTestEngine(t *testing.T) (*engine.Engine, func()) {
	tmpDir, err := os.MkdirTemp("", "vortexlogs-http-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	eng, err := engine.Open(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open engine: %v", err)
	}

	cleanup := func() {
		eng.Close()
		os.RemoveAll(tmpDir)
	}

	return eng, cleanup
}

func TestHTTPServerIngestAndQuery(t *testing.T) {
	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	srv := NewHTTPServer(":0", eng, nil)

	// 1. Ingest JSON array
	logsJSON := `[
		{"service":"auth-api","level":"ERROR","message":"Invalid token signature","labels":{"env":"prod"}},
		{"service":"payment-api","level":"INFO","message":"Invoice paid","labels":{"env":"prod"}}
	]`
	req := httptest.NewRequest("POST", "/api/v1/ingest", bytes.NewReader([]byte(logsJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleIngest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on ingest, got %d: %s", w.Code, w.Body.String())
	}

	// 2. Query via GET
	reqQuery := httptest.NewRequest("GET", "/api/v1/query?service=auth-api&level=ERROR", nil)
	wQuery := httptest.NewRecorder()
	srv.handleQuery(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on query, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var res engine.QueryResult
	if err := json.Unmarshal(wQuery.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to unmarshal query result: %v", err)
	}
	if len(res.Entries) == 0 {
		t.Fatalf("expected at least 1 matching entry, got 0")
	}
	if res.Entries[0].Service != "auth-api" {
		t.Errorf("expected service 'auth-api', got %q", res.Entries[0].Service)
	}

	// 3. Query via POST JSON body
	queryBody := `{"service":"payment-api","level":"INFO","search":"Invoice"}`
	reqPostQuery := httptest.NewRequest("POST", "/api/v1/query", bytes.NewReader([]byte(queryBody)))
	reqPostQuery.Header.Set("Content-Type", "application/json")
	wPostQuery := httptest.NewRecorder()
	srv.handleQuery(wPostQuery, reqPostQuery)

	if wPostQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on POST query, got %d: %s", wPostQuery.Code, wPostQuery.Body.String())
	}

	var postRes engine.QueryResult
	if err := json.Unmarshal(wPostQuery.Body.Bytes(), &postRes); err != nil {
		t.Fatalf("failed to unmarshal POST query result: %v", err)
	}
	if len(postRes.Entries) == 0 {
		t.Fatalf("expected at least 1 matching entry for POST query, got 0")
	}

	// 4. Test Stats Endpoint
	reqStats := httptest.NewRequest("GET", "/api/v1/stats", nil)
	wStats := httptest.NewRecorder()
	srv.handleStats(wStats, reqStats)

	if wStats.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on stats, got %d", wStats.Code)
	}
}
