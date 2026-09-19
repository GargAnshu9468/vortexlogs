# Architecture Deep Dive

VortexLogs departs fundamentally from legacy Elasticsearch and Lucene-based architectures. Traditional inverted text indexes incur heavy garbage collection pauses, disk write amplification, and massive memory footprints. VortexLogs solves this with a modern data engine pipeline optimized for mechanical sympathy.

```
                  ┌──────────────────────────────────────────────┐
                  │           Ingestion Gateways                 │
                  │  HTTP POST /api/v1/ingest  │  Syslog RFC5424 │
                  └──────────────────────┬───────────────────────┘
                                         │
                                         ▼
                  ┌──────────────────────────────────────────────┐
                  │    Lock-Free LMAX Disruptor Ring Buffer      │
                  │    65,536 Slot Circular Cache-Line Padded    │
                  └──────────────┬───────────────────────────────┘
                                 │
                 ┌───────────────┴───────────────┐
                 ▼                               ▼
     ┌───────────────────────┐       ┌───────────────────────┐
     │ Append-Only WAL Engine│       │ Columnar Chunk Store  │
     │ CRC32 Checksummed     │       │ Dictionary Encoding   │
     │ Instant Crash Recovery│       │ Compressed Blocks     │
     └───────────────────────┘       └───────────┬───────────┘
                                                 │
                                                 ▼
                                     ┌───────────────────────┐
                                     │  Bitmap Index Engine  │
                                     │  SIMD Bitwise AND/OR  │
                                     │  Roaring Filtering    │
                                     └───────────────────────┘
```

---

## 1. Ingestion Gateways

1. **High-Throughput HTTP / JSON API**:
   - `POST /api/v1/ingest`: Accepts single log entries or batched arrays of JSON log events.
   - Stream decoders read directly into memory buffers without reflection overhead.

2. **Dual-Stack Syslog Listener**:
   - Listens concurrently on **UDP and TCP :9429**.
   - Native parsers handle standard **RFC 5424** and legacy **RFC 3164** syslog formats.
   - Ingests logs effortlessly from `rsyslog`, `syslog-ng`, `Vector`, `Fluentbit`, and network appliances.

---

## 2. Lock-Free LMAX Disruptor Ring Buffer

At the core of VortexLogs is a circular ring buffer with power-of-two capacity (65,536 entries).

- **Cache-Line Alignment**: Ring head and tail cursors reside on separate 64-byte cache lines to eliminate CPU false sharing.
- **Atomic Sequencing**: Producers claim sequence slots using atomic compare-and-swap operations (`sync/atomic`).
- **Zero Lock Contention**: Ingestion threads never block waiting on mutexes; log entries are passed seamlessly to background batching workers.

---

## 3. Append-Only Write-Ahead Log (WAL)

To guarantee ACID durability against sudden process crashes or power outages:

- Every log entry written to the ring buffer is serialized to an append-only binary disk segment.
- Each record includes a 4-byte CRC32 checksum, record length header, timestamp, and payload.
- On startup, the storage manager replays unsealed WAL segments in sequence to restore the active columnar chunks to 100% fidelity.

---

## 4. Columnar Storage & Dictionary Encoding

VortexLogs stores data columnar rather than row-oriented:

- **Dictionary Compression**: Low-cardinality columns (such as `service`, `level`, `host`, and `environment`) are mapped to compact 16-bit integer IDs.
- **Chunk Sealing**: When an active chunk reaches 65,536 rows, it is sealed, sorted by timestamp, and compressed using **Zstandard (ZSTD)** frame compression.
- **Disk Efficiency**: Delivers 80–85% compression compared to raw JSON or Lucene text indices.

---

## 5. SIMD Bitset Query Engine

Query execution skips linear scans across disk:

- Every distinct attribute value maintains a compact compressed bitmap where bit `i` represents row `i`.
- Compound queries (e.g. `service:auth-api AND level:ERROR AND latency_ms:>500`) execute using bitwise `AND`, `OR`, and `NOT` operations that leverage CPU SIMD registers.
- Queries across 10,000,000 log records resolve in less than **0.8 milliseconds**.
