package engine

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

func TestEngineIngestAndQuery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vortexlogs_engine_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng, err := Open(tempDir)
	if err != nil {
		t.Fatalf("failed to open engine: %v", err)
	}
	defer eng.Close()

	now := time.Now().UnixNano()
	for i := 0; i < 500; i++ {
		lvl := storage.LevelInfo
		if i%5 == 0 {
			lvl = storage.LevelError
		}

		entry := &storage.Entry{
			Timestamp: now + int64(i*1000000), // 1ms intervals
			Level:     lvl,
			Service:   "payment-service",
			Message:   fmt.Sprintf("Checkout request #%d processed", i),
			Labels: map[string]string{
				"cluster": "us-east",
			},
		}

		if err := eng.Ingest(entry); err != nil {
			t.Fatalf("ingest error at %d: %v", i, err)
		}
	}

	// Allow ingest worker to drain
	time.Sleep(100 * time.Millisecond)

	// Query errors
	res, err := eng.Query(QueryRequest{
		StartTime: now - int64(time.Minute),
		EndTime:   now + int64(time.Hour),
		Level:     storage.LevelError,
		Limit:     100,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if res.TotalHits != 100 {
		t.Fatalf("expected 100 error hits, got %d", res.TotalHits)
	}
	if len(res.Entries) != 100 {
		t.Fatalf("expected 100 entries returned, got %d", len(res.Entries))
	}
	if len(res.Histogram) == 0 {
		t.Fatalf("expected non-empty histogram")
	}

	// Query with substring filter
	resSub, err := eng.Query(QueryRequest{
		StartTime:       now - int64(time.Minute),
		EndTime:         now + int64(time.Hour),
		SubstringFilter: "Checkout request #42 processed",
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("substring query failed: %v", err)
	}
	if resSub.TotalHits != 1 {
		t.Fatalf("expected 1 hit for #42, got %d", resSub.TotalHits)
	}
}
