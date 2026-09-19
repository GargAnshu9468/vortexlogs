package engine

import (
	"sort"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

// QueryRequest specifies search criteria, time boundaries, and aggregation limits.
type QueryRequest struct {
	StartTime       int64             `json:"start_time"` // Nanoseconds
	EndTime         int64             `json:"end_time"`   // Nanoseconds
	Level           storage.Level     `json:"level"`
	LabelSelectors  map[string]string `json:"labels"`
	SubstringFilter string            `json:"search"`
	Limit           int               `json:"limit"`
	HistogramBuckets int              `json:"histogram_buckets"`
}

// HistogramBucket represents a time-window slice with counts by severity.
type HistogramBucket struct {
	Timestamp int64 `json:"timestamp"`
	Total     int   `json:"total"`
	Debug     int   `json:"debug"`
	Info      int   `json:"info"`
	Warn      int   `json:"warn"`
	Error     int   `json:"error"`
	Fatal     int   `json:"fatal"`
}

// QueryResult holds matched log records and histogram aggregations.
type QueryResult struct {
	Entries   []*storage.Entry   `json:"entries"`
	Histogram []HistogramBucket `json:"histogram"`
	TotalHits int                `json:"total_hits"`
	DurationMs float64           `json:"duration_ms"`
}

// Query executes a sub-millisecond search across active and sealed columnar chunks.
func (e *Engine) Query(req QueryRequest) (*QueryResult, error) {
	startTimer := time.Now()
	_ = e.Flush()

	if req.EndTime == 0 {
		req.EndTime = time.Now().UnixNano()
	}
	if req.StartTime == 0 {
		req.StartTime = req.EndTime - (24 * time.Hour).Nanoseconds()
	}
	if req.Limit <= 0 || req.Limit > 10000 {
		req.Limit = 100
	}
	if req.HistogramBuckets <= 0 {
		req.HistogramBuckets = 30
	} else if req.HistogramBuckets > 300 {
		req.HistogramBuckets = 300
	}

	e.mu.RLock()
	allChunks := make([]*storage.Chunk, 0, len(e.chunks)+1)
	allChunks = append(allChunks, e.chunks...)
	if e.activeChunk != nil {
		allChunks = append(allChunks, e.activeChunk)
	}
	e.mu.RUnlock()

	// Initialize histogram buckets
	timeSpan := req.EndTime - req.StartTime
	if timeSpan <= 0 {
		timeSpan = int64(time.Minute)
	}
	bucketWidth := timeSpan / int64(req.HistogramBuckets)
	if bucketWidth <= 0 {
		bucketWidth = 1
	}
	histogram := make([]HistogramBucket, req.HistogramBuckets)
	for i := 0; i < req.HistogramBuckets; i++ {
		histogram[i].Timestamp = req.StartTime + int64(i)*bucketWidth
	}

	var matchedEntries []*storage.Entry
	totalHits := 0

	for _, chunk := range allChunks {
		rowIndexes := chunk.Matches(req.StartTime, req.EndTime, req.Level, req.LabelSelectors)
		if len(rowIndexes) == 0 {
			continue
		}

		entries := chunk.FetchEntries(rowIndexes, req.SubstringFilter)
		totalHits += len(entries)

		for _, entry := range entries {
			// Aggregate into histogram bucket
			bucketIdx := int((entry.Timestamp - req.StartTime) / bucketWidth)
			if bucketIdx >= 0 && bucketIdx < req.HistogramBuckets {
				histogram[bucketIdx].Total++
				switch entry.Level {
				case storage.LevelDebug:
					histogram[bucketIdx].Debug++
				case storage.LevelInfo:
					histogram[bucketIdx].Info++
				case storage.LevelWarn:
					histogram[bucketIdx].Warn++
				case storage.LevelError:
					histogram[bucketIdx].Error++
				case storage.LevelFatal:
					histogram[bucketIdx].Fatal++
				}
			}

			if len(matchedEntries) < req.Limit {
				matchedEntries = append(matchedEntries, entry)
			}
		}
	}

	// Sort matched entries descending by timestamp (newest first)
	sort.Slice(matchedEntries, func(i, j int) bool {
		return matchedEntries[i].Timestamp > matchedEntries[j].Timestamp
	})

	return &QueryResult{
		Entries:    matchedEntries,
		Histogram:  histogram,
		TotalHits:  totalHits,
		DurationMs: float64(time.Since(startTimer).Microseconds()) / 1000.0,
	}, nil
}
