package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
	"github.com/GargAnshu9468/vortexlogs/internal/storage"
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

func TestHTTPServerIngestBinary(t *testing.T) {
	eng, cleanup := setupTestEngine(t)
	defer cleanup()

	srv := NewHTTPServer(":0", eng, nil)

	// Build binary frame for 2 records
	var buf bytes.Buffer
	// Count = 2
	var count uint32 = 2
	_ = binary.Write(&buf, binary.LittleEndian, count)

	// Record 1
	ts := time.Now().UnixNano()
	_ = binary.Write(&buf, binary.LittleEndian, ts)
	_ = buf.WriteByte(byte(storage.LevelError))
	srv1 := "billing-svc"
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(srv1)))
	buf.WriteString(srv1)
	msg1 := "Card charge failed due to expired token"
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(msg1)))
	buf.WriteString(msg1)

	// Record 2
	_ = binary.Write(&buf, binary.LittleEndian, ts+1000)
	_ = buf.WriteByte(byte(storage.LevelInfo))
	srv2 := "billing-svc"
	_ = binary.Write(&buf, binary.LittleEndian, uint16(len(srv2)))
	buf.WriteString(srv2)
	msg2 := "Refund issued successfully"
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(msg2)))
	buf.WriteString(msg2)

	req := httptest.NewRequest("POST", "/api/v1/ingest/binary", &buf)
	w := httptest.NewRecorder()
	srv.handleIngestBinary(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on binary ingest, got %d: %s", w.Code, w.Body.String())
	}

	// Verify query returns the binary ingested logs
	reqQuery := httptest.NewRequest("GET", "/api/v1/query?service=billing-svc&level=ERROR", nil)
	wQuery := httptest.NewRecorder()
	srv.handleQuery(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on query, got %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var res engine.QueryResult
	if err := json.Unmarshal(wQuery.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode query result: %v", err)
	}

	if len(res.Entries) != 1 {
		t.Fatalf("expected 1 error entry, got %d", len(res.Entries))
	}
	if res.Entries[0].Message != msg1 {
		t.Errorf("expected message %q, got %q", msg1, res.Entries[0].Message)
	}
}
