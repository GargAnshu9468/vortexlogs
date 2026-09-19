package storage

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	encoderPool = sync.Pool{
		New: func() any {
			enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
			return enc
		},
	}
	decoderPool = sync.Pool{
		New: func() any {
			dec, _ := zstd.NewReader(nil)
			return dec
		},
	}
)

// Bitset implements an efficient roaring-style bitmap for columnar bitwise index filtering.
type Bitset []uint64

// Set sets the bit at index i to 1.
func (b *Bitset) Set(i int) {
	word := i / 64
	bit := uint(i % 64)
	for len(*b) <= word {
		*b = append(*b, 0)
	}
	(*b)[word] |= 1 << bit
}

// Test checks whether the bit at index i is 1.
func (b Bitset) Test(i int) bool {
	word := i / 64
	if word >= len(b) {
		return false
	}
	bit := uint(i % 64)
	return (b[word] & (1 << bit)) != 0
}

// And returns the bitwise intersection of two bitsets.
func (b Bitset) And(other Bitset) Bitset {
	minLen := len(b)
	if len(other) < minLen {
		minLen = len(other)
	}
	res := make(Bitset, minLen)
	for i := 0; i < minLen; i++ {
		res[i] = b[i] & other[i]
	}
	return res
}

// Chunk represents a columnar block of log entries with ZSTD compression and bitset index.
type Chunk struct {
	mu           sync.RWMutex
	ID           uint64
	MinTimestamp int64
	MaxTimestamp int64
	Count        int

	// Columnar Vectors
	Timestamps []int64 // Delta-encoded timestamps
	Levels     []Level // Dictionary encoded levels
	Services   []string
	Messages   []string
	RawJSONs   [][]byte

	// Inverted Index: Label "key=value" -> Bitset
	LabelIndex map[string]*Bitset
	LevelIndex map[Level]*Bitset

	// Compressed serialized state
	isSealed       bool
	compressedData []byte
	rawBytesCount  int
}

// NewChunk initializes a new active columnar chunk.
func NewChunk(id uint64) *Chunk {
	return &Chunk{
		ID:         id,
		LabelIndex: make(map[string]*Bitset),
		LevelIndex: make(map[Level]*Bitset),
	}
}

// Append appends a log entry into the columnar chunk vectors.
func (c *Chunk) Append(entry *Entry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := c.Count
	c.Count++

	if c.MinTimestamp == 0 || entry.Timestamp < c.MinTimestamp {
		c.MinTimestamp = entry.Timestamp
	}
	if entry.Timestamp > c.MaxTimestamp {
		c.MaxTimestamp = entry.Timestamp
	}

	c.Timestamps = append(c.Timestamps, entry.Timestamp)
	c.Levels = append(c.Levels, entry.Level)
	c.Services = append(c.Services, entry.Service)
	c.Messages = append(c.Messages, entry.Message)
	c.RawJSONs = append(c.RawJSONs, entry.RawJSON)

	// Update raw byte counter for compression ratio tracking
	c.rawBytesCount += len(entry.Service) + len(entry.Message) + len(entry.RawJSON) + 24

	// Index Service
	srvTag := fmt.Sprintf("service=%s", strings.ToLower(entry.Service))
	if _, ok := c.LabelIndex[srvTag]; !ok {
		c.LabelIndex[srvTag] = &Bitset{}
	}
	c.LabelIndex[srvTag].Set(idx)

	// Index Custom Labels
	for k, v := range entry.Labels {
		tag := fmt.Sprintf("%s=%s", strings.ToLower(k), strings.ToLower(v))
		if _, ok := c.LabelIndex[tag]; !ok {
			c.LabelIndex[tag] = &Bitset{}
		}
		c.LabelIndex[tag].Set(idx)
	}

	// Index Level
	if _, ok := c.LevelIndex[entry.Level]; !ok {
		c.LevelIndex[entry.Level] = &Bitset{}
	}
	c.LevelIndex[entry.Level].Set(idx)

	return nil
}

// Seal compresses the chunk into a ZSTD block and releases in-memory raw payload slices.
func (c *Chunk) Seal() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isSealed || c.Count == 0 {
		return nil
	}

	// Serialize columnar messages into a single byte stream for ZSTD compression
	var buf bytes.Buffer
	for i := 0; i < c.Count; i++ {
		// [4 bytes msg length] + [msg] + [4 bytes json length] + [json]
		binary.Write(&buf, binary.LittleEndian, uint32(len(c.Messages[i])))
		buf.WriteString(c.Messages[i])
		binary.Write(&buf, binary.LittleEndian, uint32(len(c.RawJSONs[i])))
		buf.Write(c.RawJSONs[i])
	}

	encoder := encoderPool.Get().(*zstd.Encoder)
	defer encoderPool.Put(encoder)

	c.compressedData = encoder.EncodeAll(buf.Bytes(), make([]byte, 0, buf.Len()/4))
	c.isSealed = true

	// Clear uncompressed payload slices to free memory while keeping index & metadata hot
	c.Messages = nil
	c.RawJSONs = nil

	return nil
}

// CompressionRatio returns the space savings percentage achieved by ZSTD.
func (c *Chunk) CompressionRatio() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isSealed || len(c.compressedData) == 0 || c.rawBytesCount == 0 {
		return 0.0
	}
	return (1.0 - (float64(len(c.compressedData)) / float64(c.rawBytesCount))) * 100.0
}

// Matches evaluates query criteria and returns matching entry row indexes.
func (c *Chunk) Matches(startTime, endTime int64, level Level, labelSelectors map[string]string) []int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.Count == 0 {
		return nil
	}
	// Quick time-range pruning
	if endTime < c.MinTimestamp || startTime > c.MaxTimestamp {
		return nil
	}

	// Start with candidate bitmap if filters exist
	var candidateBits Bitset
	hasFilter := false

	// Filter by Level
	if level != LevelUnknown {
		bs, ok := c.LevelIndex[level]
		if !ok {
			return nil
		}
		candidateBits = *bs
		hasFilter = true
	}

	// Filter by Label Selectors
	for k, v := range labelSelectors {
		tag := fmt.Sprintf("%s=%s", strings.ToLower(k), strings.ToLower(v))
		bs, ok := c.LabelIndex[tag]
		if !ok {
			return nil
		}
		if !hasFilter {
			candidateBits = *bs
			hasFilter = true
		} else {
			candidateBits = candidateBits.And(*bs)
		}
	}

	var matchingIndexes []int
	for i := 0; i < c.Count; i++ {
		ts := c.Timestamps[i]
		if ts < startTime || ts > endTime {
			continue
		}
		if hasFilter && !candidateBits.Test(i) {
			continue
		}
		matchingIndexes = append(matchingIndexes, i)
	}

	return matchingIndexes
}

// FetchEntries decompresses and reconstructs entries for the given matched row indexes.
func (c *Chunk) FetchEntries(indexes []int, substringFilter string) []*Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(indexes) == 0 {
		return nil
	}

	var messages []string
	var rawJSONs [][]byte

	if !c.isSealed {
		messages = c.Messages
		rawJSONs = c.RawJSONs
	} else {
		// Decompress payload block
		decoder := decoderPool.Get().(*zstd.Decoder)
		defer decoderPool.Put(decoder)

		decompressed, err := decoder.DecodeAll(c.compressedData, nil)
		if err != nil {
			return nil
		}

		reader := bytes.NewReader(decompressed)
		messages = make([]string, c.Count)
		rawJSONs = make([][]byte, c.Count)

		for i := 0; i < c.Count; i++ {
			var msgLen uint32
			if err := binary.Read(reader, binary.LittleEndian, &msgLen); err != nil {
				break
			}
			msgBuf := make([]byte, msgLen)
			if _, err := io.ReadFull(reader, msgBuf); err != nil {
				break
			}
			messages[i] = string(msgBuf)

			var jsonLen uint32
			if err := binary.Read(reader, binary.LittleEndian, &jsonLen); err != nil {
				break
			}
			if jsonLen > 0 {
				jsonBuf := make([]byte, jsonLen)
				if _, err := io.ReadFull(reader, jsonBuf); err != nil {
					break
				}
				rawJSONs[i] = jsonBuf
			}
		}
	}

	subLower := strings.ToLower(substringFilter)
	var results []*Entry

	for _, idx := range indexes {
		if idx >= c.Count {
			continue
		}
		msg := messages[idx]
		if subLower != "" && !strings.Contains(strings.ToLower(msg), subLower) {
			continue
		}

		e := &Entry{
			Timestamp: c.Timestamps[idx],
			Level:     c.Levels[idx],
			Service:   c.Services[idx],
			Message:   msg,
			RawJSON:   rawJSONs[idx],
		}
		results = append(results, e)
	}

	return results
}
