package wal

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

var (
	ErrCorruptWAL = errors.New("wal: data corruption detected")
)

const (
	maxEntrySize = 16 * 1024 * 1024 // 16 MB max log entry
)

// WAL manages append-only sequential durability logging.
type WAL struct {
	mu        sync.Mutex
	file      *os.File
	bufWriter *bufio.Writer
	dir       string
	filename  string
	table     *crc32.Table
}

// Open opens or creates a WAL in the specified directory.
func Open(dir string) (*WAL, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("wal: failed to create dir: %w", err)
	}

	filename := filepath.Join(dir, "vortexlogs.wal")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("wal: failed to open file: %w", err)
	}

	return &WAL{
		file:      file,
		bufWriter: bufio.NewWriterSize(file, 256*1024), // 256KB buffer for ultra-fast group writes
		dir:       dir,
		filename:  filename,
		table:     crc32.MakeTable(crc32.Castagnoli),
	}, nil
}

// Append writes a log entry to the WAL with CRC32 checksum and length framing.
func (w *WAL) Append(entry *storage.Entry) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	data, err := entry.MarshalBinary()
	if err != nil {
		return fmt.Errorf("wal: marshal error: %w", err)
	}

	if len(data) > maxEntrySize {
		return fmt.Errorf("wal: entry size %d exceeds maximum %d", len(data), maxEntrySize)
	}

	checksum := crc32.Checksum(data, w.table)

	// Framing: [4 bytes CRC32] + [4 bytes Length] + [Payload]
	header := make([]byte, 8)
	binary.LittleEndian.PutUint32(header[0:4], checksum)
	binary.LittleEndian.PutUint32(header[4:8], uint32(len(data)))

	if _, err := w.bufWriter.Write(header); err != nil {
		return fmt.Errorf("wal: write header error: %w", err)
	}
	if _, err := w.bufWriter.Write(data); err != nil {
		return fmt.Errorf("wal: write payload error: %w", err)
	}

	if w.bufWriter.Buffered() >= 128*1024 {
		_ = w.bufWriter.Flush()
	}

	return nil
}

// Sync commits written WAL records to physical disk storage.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bufWriter != nil {
		if err := w.bufWriter.Flush(); err != nil {
			return err
		}
	}
	return w.file.Sync()
}

// Recover reads all valid entries from the WAL file.
func (w *WAL) Recover() ([]*storage.Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bufWriter != nil {
		_ = w.bufWriter.Flush()
	}

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("wal: seek start error: %w", err)
	}

	var entries []*storage.Entry
	header := make([]byte, 8)
	var lastValidOffset int64

	for {
		curOffset, err := w.file.Seek(0, io.SeekCurrent)
		if err != nil {
			break
		}

		_, err = io.ReadFull(w.file, header)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("wal: read header error: %w", err)
		}

		expectedChecksum := binary.LittleEndian.Uint32(header[0:4])
		length := binary.LittleEndian.Uint32(header[4:8])

		if length > maxEntrySize {
			// Corrupted frame detected, truncate at last valid point
			break
		}

		payload := make([]byte, length)
		if _, err := io.ReadFull(w.file, payload); err != nil {
			// Incomplete write at end of file, stop recovery
			break
		}

		actualChecksum := crc32.Checksum(payload, w.table)
		if actualChecksum != expectedChecksum {
			// Checksum mismatch, corrupted record
			break
		}

		entry := &storage.Entry{}
		if err := entry.UnmarshalBinary(payload); err != nil {
			break
		}

		entries = append(entries, entry)
		lastValidOffset = curOffset + 8 + int64(length)
	}

	// Truncate any trailing corrupted or incomplete write from sudden power cuts/crash
	if err := w.file.Truncate(lastValidOffset); err != nil {
		return nil, fmt.Errorf("wal: truncate to last valid offset error: %w", err)
	}
	if _, err := w.file.Seek(lastValidOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("wal: seek to last valid offset error: %w", err)
	}

	return entries, nil
}

// Truncate clears the WAL after chunks have been safely flushed to persistent columnar storage.
func (w *WAL) Truncate() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.bufWriter != nil {
		_ = w.bufWriter.Flush()
	}
	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("wal: truncate error: %w", err)
	}
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("wal: seek start error: %w", err)
	}
	if w.bufWriter != nil {
		w.bufWriter.Reset(w.file)
	}
	return w.file.Sync()
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.bufWriter != nil {
		_ = w.bufWriter.Flush()
	}
	return w.file.Close()
}
