package wal

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

func TestWALAppendAndRecovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vortexlogs_wal_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	w, err := Open(tempDir)
	if err != nil {
		t.Fatalf("failed to open wal: %v", err)
	}

	testEntries := []*storage.Entry{
		{
			ID:        1,
			Timestamp: time.Now().UnixNano(),
			Level:     storage.LevelInfo,
			Service:   "auth-service",
			Message:   "User signed in",
			Labels:    map[string]string{"env": "test"},
		},
		{
			ID:        2,
			Timestamp: time.Now().UnixNano(),
			Level:     storage.LevelError,
			Service:   "db-service",
			Message:   "Deadlock detected on table accounts",
			Labels:    map[string]string{"shard": "03"},
		},
	}

	for _, e := range testEntries {
		if err := w.Append(e); err != nil {
			t.Fatalf("failed to append to wal: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close wal: %v", err)
	}

	// Reopen WAL and test recovery
	w2, err := Open(tempDir)
	if err != nil {
		t.Fatalf("failed to reopen wal: %v", err)
	}
	defer w2.Close()

	recovered, err := w2.Recover()
	if err != nil {
		t.Fatalf("failed to recover from wal: %v", err)
	}

	if len(recovered) != 2 {
		t.Fatalf("expected 2 recovered entries, got %d", len(recovered))
	}

	if recovered[0].Message != "User signed in" || recovered[1].Message != "Deadlock detected on table accounts" {
		t.Fatalf("recovered messages do not match: %+v", recovered)
	}
	if recovered[1].Labels["shard"] != "03" {
		t.Fatalf("recovered label mismatch: %v", recovered[1].Labels)
	}

	// Test truncate
	if err := w2.Truncate(); err != nil {
		t.Fatalf("failed to truncate wal: %v", err)
	}

	stat, err := os.Stat(filepath.Join(tempDir, "vortexlogs.wal"))
	if err != nil {
		t.Fatalf("failed to stat wal file: %v", err)
	}
	if stat.Size() != 0 {
		t.Fatalf("expected 0 bytes after reset, got %d", stat.Size())
	}
}
