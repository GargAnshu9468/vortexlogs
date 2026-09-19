package storage

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Level represents the severity level of a log entry.
type Level uint8

const (
	LevelUnknown Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of a log severity level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a case-insensitive string into a Level.
func ParseLevel(s string) Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG", "DBG", "TRACE":
		return LevelDebug
	case "INFO", "INF", "NOTICE":
		return LevelInfo
	case "WARN", "WARNING":
		return LevelWarn
	case "ERROR", "ERR":
		return LevelError
	case "FATAL", "CRITICAL", "EMERGENCY", "PANIC":
		return LevelFatal
	default:
		return LevelInfo
	}
}

// Entry represents a high-performance log event in VortexLogs.
type Entry struct {
	ID        uint64            `json:"id"`
	Timestamp int64             `json:"timestamp"` // Unix timestamp in nanoseconds
	Level     Level             `json:"level"`
	Service   string            `json:"service"`
	Message   string            `json:"message"`
	Labels    map[string]string `json:"labels,omitempty"`
	RawJSON   json.RawMessage   `json:"data,omitempty"`
}

// NewEntry creates a normalized log entry with current nanosecond timestamp.
func NewEntry(level Level, service, message string, labels map[string]string) *Entry {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &Entry{
		Timestamp: time.Now().UnixNano(),
		Level:     level,
		Service:   service,
		Message:   message,
		Labels:    labels,
	}
}

// MarshalBinary serializes an entry into a compact binary format for WAL/storage.
// Layout:
// [8 bytes: ID]
// [8 bytes: Timestamp (int64)]
// [1 byte: Level]
// [2 bytes: Service length] + [N bytes: Service string]
// [4 bytes: Message length] + [N bytes: Message string]
// [2 bytes: Number of labels]
//   For each label:
//     [2 bytes: Key length] + [N bytes: Key]
//     [2 bytes: Value length] + [N bytes: Value]
// [4 bytes: RawJSON length] + [N bytes: RawJSON]
func (e *Entry) MarshalBinary() ([]byte, error) {
	srvBytes := []byte(e.Service)
	msgBytes := []byte(e.Message)

	size := 8 + 8 + 1 + 2 + len(srvBytes) + 4 + len(msgBytes) + 2
	for k, v := range e.Labels {
		size += 2 + len(k) + 2 + len(v)
	}
	size += 4 + len(e.RawJSON)

	buf := make([]byte, size)
	binary.LittleEndian.PutUint64(buf[0:8], e.ID)
	binary.LittleEndian.PutUint64(buf[8:16], uint64(e.Timestamp))
	buf[16] = byte(e.Level)

	offset := 17
	binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(srvBytes)))
	offset += 2
	copy(buf[offset:], srvBytes)
	offset += len(srvBytes)

	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(msgBytes)))
	offset += 4
	copy(buf[offset:], msgBytes)
	offset += len(msgBytes)

	binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(e.Labels)))
	offset += 2
	for k, v := range e.Labels {
		kBytes := []byte(k)
		vBytes := []byte(v)

		binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(kBytes)))
		offset += 2
		copy(buf[offset:], kBytes)
		offset += len(kBytes)

		binary.LittleEndian.PutUint16(buf[offset:offset+2], uint16(len(vBytes)))
		offset += 2
		copy(buf[offset:], vBytes)
		offset += len(vBytes)
	}

	binary.LittleEndian.PutUint32(buf[offset:offset+4], uint32(len(e.RawJSON)))
	offset += 4
	copy(buf[offset:], e.RawJSON)

	return buf, nil
}

// UnmarshalBinary parses an entry from binary format.
func (e *Entry) UnmarshalBinary(data []byte) error {
	if len(data) < 27 { // minimum header size
		return fmt.Errorf("storage: binary data too short (%d bytes)", len(data))
	}

	e.ID = binary.LittleEndian.Uint64(data[0:8])
	e.Timestamp = int64(binary.LittleEndian.Uint64(data[8:16]))
	e.Level = Level(data[16])

	offset := 17
	srvLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	if offset+srvLen > len(data) {
		return fmt.Errorf("storage: corrupted service string length")
	}
	e.Service = string(data[offset : offset+srvLen])
	offset += srvLen

	msgLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+msgLen > len(data) {
		return fmt.Errorf("storage: corrupted message string length")
	}
	e.Message = string(data[offset : offset+msgLen])
	offset += msgLen

	labelCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2
	e.Labels = make(map[string]string, labelCount)

	for i := 0; i < labelCount; i++ {
		kLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		k := string(data[offset : offset+kLen])
		offset += kLen

		vLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		v := string(data[offset : offset+vLen])
		offset += vLen

		e.Labels[k] = v
	}

	jsonLen := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if jsonLen > 0 && offset+jsonLen <= len(data) {
		e.RawJSON = make([]byte, jsonLen)
		copy(e.RawJSON, data[offset:offset+jsonLen])
	}

	return nil
}
