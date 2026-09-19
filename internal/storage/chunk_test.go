package storage

import (
	"fmt"
	"testing"
	"time"
)

func TestChunkAppendAndBitsetSearch(t *testing.T) {
	chunk := NewChunk(1)

	now := time.Now().UnixNano()
	for i := 0; i < 1000; i++ {
		lvl := LevelInfo
		srv := "api-gateway"
		if i%10 == 0 {
			lvl = LevelError
		}
		if i%5 == 0 {
			srv = "auth-service"
		}

		entry := &Entry{
			ID:        uint64(i + 1),
			Timestamp: now + int64(i*1000),
			Level:     lvl,
			Service:   srv,
			Message:   fmt.Sprintf("Transaction %d completed successfully", i),
			Labels: map[string]string{
				"env": "prod",
			},
		}

		if err := chunk.Append(entry); err != nil {
			t.Fatalf("failed to append entry %d: %v", i, err)
		}
	}

	if chunk.Count != 1000 {
		t.Fatalf("expected count 1000, got %d", chunk.Count)
	}

	// Test bitset query: search for LevelError
	matched := chunk.Matches(0, now+int64(1000*1000), LevelError, nil)
	if len(matched) != 100 {
		t.Fatalf("expected 100 error entries, got %d", len(matched))
	}

	// Test bitset query: search for LevelError AND Service="auth-service"
	matchedAuthErrors := chunk.Matches(0, now+int64(1000*1000), LevelError, map[string]string{"service": "auth-service"})
	// Every 10th is Error, every 5th is auth-service => every 10th is BOTH (0, 10, 20... up to 990 = 100 entries)
	if len(matchedAuthErrors) != 100 {
		t.Fatalf("expected 100 auth error entries, got %d", len(matchedAuthErrors))
	}

	// Fetch entries before seal
	entries := chunk.FetchEntries(matchedAuthErrors[:5], "Transaction")
	if len(entries) != 5 {
		t.Fatalf("expected 5 fetched entries, got %d", len(entries))
	}
	if entries[0].Level != LevelError || entries[0].Service != "auth-service" {
		t.Fatalf("unexpected entry attributes: %+v", entries[0])
	}
}

func TestChunkSealAndDecompress(t *testing.T) {
	chunk := NewChunk(2)
	now := time.Now().UnixNano()

	for i := 0; i < 500; i++ {
		entry := &Entry{
			ID:        uint64(i + 1),
			Timestamp: now + int64(i*1000),
			Level:     LevelWarn,
			Service:   "billing-worker",
			Message:   fmt.Sprintf("Invoice #%d payment retried due to gateway timeout", i),
		}
		_ = chunk.Append(entry)
	}

	// Seal chunk (triggers ZSTD block compression)
	if err := chunk.Seal(); err != nil {
		t.Fatalf("failed to seal chunk: %v", err)
	}

	if !chunk.isSealed {
		t.Fatalf("expected chunk to be sealed")
	}
	if len(chunk.compressedData) == 0 {
		t.Fatalf("expected compressed data to be populated")
	}

	// Query sealed chunk
	matched := chunk.Matches(0, now+int64(500*1000), LevelWarn, nil)
	if len(matched) != 500 {
		t.Fatalf("expected 500 matches, got %d", len(matched))
	}

	// Substring search on decompressed data
	results := chunk.FetchEntries(matched, "Invoice #42 payment retried")
	if len(results) != 1 {
		t.Fatalf("expected 1 hit for Invoice #42 payment retried, got %d", len(results))
	}
	if results[0].Message != "Invoice #42 payment retried due to gateway timeout" {
		t.Fatalf("unexpected message content: %s", results[0].Message)
	}
}

func TestChunkZeroMatchFilter(t *testing.T) {
	chunk := NewChunk(3)
	now := time.Now().UnixNano()

	for i := 0; i < 50; i++ {
		entry := &Entry{
			ID:        uint64(i + 1),
			Timestamp: now + int64(i*1000),
			Level:     LevelInfo,
			Service:   "api",
			Message:   fmt.Sprintf("Log line %d", i),
			Labels:    map[string]string{"env": "staging"},
		}
		_ = chunk.Append(entry)
	}

	// 1. Filter for a level that doesn't exist in the chunk (LevelFatal)
	// MUST return 0 matches, NOT all matches!
	fatalMatches := chunk.Matches(0, now+1000000, LevelFatal, nil)
	if len(fatalMatches) != 0 {
		t.Fatalf("expected 0 matches for non-existent level, got %d", len(fatalMatches))
	}

	// 2. Filter for a label that doesn't exist
	labelMatches := chunk.Matches(0, now+1000000, LevelUnknown, map[string]string{"env": "non-existent"})
	if len(labelMatches) != 0 {
		t.Fatalf("expected 0 matches for non-existent label, got %d", len(labelMatches))
	}
}

func TestEntryUnmarshalCorruptedData(t *testing.T) {
	e := &Entry{}

	// Short byte arrays
	for i := 0; i < 27; i++ {
		garbage := make([]byte, i)
		if err := e.UnmarshalBinary(garbage); err == nil {
			t.Errorf("expected error for %d-byte truncated payload, got nil", i)
		}
	}

	// Corrupted lengths
	corruptPayload := make([]byte, 50)
	corruptPayload[17] = 255 // service length 255 exceeds 50
	if err := e.UnmarshalBinary(corruptPayload); err == nil {
		t.Errorf("expected error for corrupted service length, got nil")
	}
}
