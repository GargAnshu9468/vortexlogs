package ring

import (
	"errors"
	"sync/atomic"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

var (
	ErrRingFull  = errors.New("ring: buffer is full")
	ErrRingEmpty = errors.New("ring: buffer is empty")
)

// RingBuffer implements a high-throughput, lock-free circular buffer with cacheline isolation.
type RingBuffer struct {
	_pad0 [64]byte
	head  uint64 // Producer write cursor
	_pad1 [64]byte
	tail  uint64 // Consumer read cursor
	_pad2 [64]byte
	mask  uint64
	cap   uint64
	_pad3 [64]byte
	slots []*storage.Entry
}

// NewRingBuffer allocates a power-of-two capacity ring buffer.
func NewRingBuffer(capacity uint64) *RingBuffer {
	// Round up to nearest power of two
	if capacity < 2 {
		capacity = 2
	}
	capPow2 := uint64(1)
	for capPow2 < capacity {
		capPow2 <<= 1
	}

	return &RingBuffer{
		mask:  capPow2 - 1,
		cap:   capPow2,
		slots: make([]*storage.Entry, capPow2),
	}
}

// Offer attempts to push an entry into the ring buffer without blocking.
// Returns ErrRingFull if the buffer capacity is reached.
func (r *RingBuffer) Offer(entry *storage.Entry) error {
	for {
		head := atomic.LoadUint64(&r.head)
		tail := atomic.LoadUint64(&r.tail)

		if head-tail >= r.cap {
			return ErrRingFull
		}

		if atomic.CompareAndSwapUint64(&r.head, head, head+1) {
			r.slots[head&r.mask] = entry
			return nil
		}
	}
}

// Poll attempts to pop the next entry from the ring buffer without blocking.
// Returns ErrRingEmpty if no entries are available.
func (r *RingBuffer) Poll() (*storage.Entry, error) {
	for {
		tail := atomic.LoadUint64(&r.tail)
		head := atomic.LoadUint64(&r.head)

		if tail >= head {
			return nil, ErrRingEmpty
		}

		entry := r.slots[tail&r.mask]
		if entry == nil {
			// Producer claimed sequence but has not written the pointer yet
			return nil, ErrRingEmpty
		}

		if atomic.CompareAndSwapUint64(&r.tail, tail, tail+1) {
			r.slots[tail&r.mask] = nil // avoid GC retention
			return entry, nil
		}
	}
}

// Size returns the approximate number of entries queued in the ring.
func (r *RingBuffer) Size() uint64 {
	head := atomic.LoadUint64(&r.head)
	tail := atomic.LoadUint64(&r.tail)
	if head > tail {
		return head - tail
	}
	return 0
}

// Cap returns the total capacity of the ring buffer.
func (r *RingBuffer) Cap() uint64 {
	return r.cap
}
