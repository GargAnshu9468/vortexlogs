package ring

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer(65536)

	const producers = 4
	const itemsPerProducer = 10000
	var wg sync.WaitGroup
	wg.Add(producers)

	for p := 0; p < producers; p++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < itemsPerProducer; i++ {
				e := &storage.Entry{
					ID:        uint64(id*itemsPerProducer + i),
					Timestamp: time.Now().UnixNano(),
					Message:   "metric event",
				}
				for rb.Offer(e) != nil {
					// wait if buffer is full
					time.Sleep(50 * time.Microsecond)
				}
			}
		}(p)
	}

	var totalConsumed atomic.Int64
	done := make(chan struct{})
	var consumerWg sync.WaitGroup
	consumerWg.Add(1)

	go func() {
		defer consumerWg.Done()
		for {
			select {
			case <-done:
				// drain remainder
				for {
					if _, err := rb.Poll(); err != nil {
						return
					}
					totalConsumed.Add(1)
				}
			default:
				if _, err := rb.Poll(); err == nil {
					totalConsumed.Add(1)
				} else {
					time.Sleep(10 * time.Microsecond)
				}
			}
		}
	}()

	wg.Wait()
	close(done)
	consumerWg.Wait()

	expected := int64(producers * itemsPerProducer)
	if totalConsumed.Load() != expected {
		t.Fatalf("expected %d consumed items, got %d", expected, totalConsumed.Load())
	}
}
