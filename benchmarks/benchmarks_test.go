package benchmarks

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
	"github.com/GargAnshu9468/vortexlogs/internal/ring"
	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

func BenchmarkRingBufferPushPop(b *testing.B) {
	rb := ring.NewRingBuffer(262144)
	e := &storage.Entry{ID: 1, Message: "benchmark test log message"}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = rb.Offer(e)
		_, _ = rb.Poll()
	}
}

func BenchmarkChunkAppend(b *testing.B) {
	chunk := storage.NewChunk(1)
	entry := &storage.Entry{
		Timestamp: time.Now().UnixNano(),
		Level:     storage.LevelInfo,
		Service:   "bench-service",
		Message:   "HTTP GET /api/v1/resource status=200 duration=1.2ms client=10.0.0.1",
		Labels: map[string]string{
			"env": "production",
			"dc":  "us-east-1",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if chunk.Count >= 100000 {
			chunk = storage.NewChunk(uint64(i))
		}
		_ = chunk.Append(entry)
	}
}

func BenchmarkBitsetQuery100K(b *testing.B) {
	chunk := storage.NewChunk(1)
	now := time.Now().UnixNano()

	for i := 0; i < 100000; i++ {
		lvl := storage.LevelInfo
		srv := "api-gateway"
		if i%20 == 0 {
			lvl = storage.LevelError
		}
		if i%10 == 0 {
			srv = "auth-service"
		}

		_ = chunk.Append(&storage.Entry{
			ID:        uint64(i + 1),
			Timestamp: now + int64(i*1000),
			Level:     lvl,
			Service:   srv,
			Message:   fmt.Sprintf("User action %d processed in cluster", i),
		})
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		matches := chunk.Matches(0, now+int64(100000*1000), storage.LevelError, map[string]string{"service": "auth-service"})
		if len(matches) == 0 {
			b.Fatal("unexpected zero matches")
		}
	}
}

func BenchmarkEngineEndToEndIngest(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "vortexlogs_bench_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	eng, err := engine.Open(tempDir)
	if err != nil {
		b.Fatal(err)
	}
	defer eng.Close()

	entry := &storage.Entry{
		Level:   storage.LevelInfo,
		Service: "order-service",
		Message: "Order placed successfully for customer_id=81928 items=3 amount=149.99",
		Labels: map[string]string{
			"env": "prod",
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = eng.Ingest(entry)
	}
}
