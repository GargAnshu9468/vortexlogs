# Why We Built VortexLogs: Ingesting 2.31M Logs/Second with Zero JVM Overhead in Pure Go

Running modern cloud infrastructure produces gigabytes to terabytes of log output every day. Yet, for years, the standard logging tier has demanded a heavy toll:
- **Elasticsearch/Opensearch**: Massive JVM heaps (often 32GB+), unpredictable GC pauses, runaway disk inflation from inverted indexes, and complex multi-node topology maintenance.
- **Grafana Loki**: Efficient index-less chunking, but high query latencies when scanning non-indexed attributes across large time spans.
- **ClickHouse**: Exceptional throughput on columnar analytics, but complex cluster topology and heavy memory requirements for simple log tailing.

We asked a fundamental systems engineering question:
> **What if a log engine was engineered with mechanical sympathy from the metal up — achieving 2.31M+ logs/sec Native Binary ingestion (1.27M+ logs/sec HTTP REST), sub-millisecond bitset queries, 98.37% columnar compression, and an embedded real-time web UI, all compiled into an 11 MB standalone binary?**

That vision is **VortexLogs**.

---

## 🏛️ The Engineering Pillars

### 1. Lock-Free LMAX Disruptor Ingestion
Mutex contention under high concurrency kills CPU efficiency. VortexLogs implements an atomic circular ring buffer with cache-line-padded sequence cursors. Log ingestion writes proceed without acquiring a single lock, saturating network interfaces effortlessly at **2,312,887 logs/sec** via Native Binary Protocol and **1,271,812 logs/sec** over HTTP REST.

### 2. Append-Only Write-Ahead Log (WAL)
Durability is non-negotiable. Every entry committed to the ring buffer is synchronously flushed to a lightweight, CRC32-checksummed append-only binary segment on disk (**1,050,000 writes/sec**). Crash recovery on restart takes only milliseconds.

### 3. Columnar Chunk Storage & Zstandard Compression
Rows are batched into 64K-record columnar chunks. Low-cardinality metadata (service names, severity levels, hostnames) is dictionary-encoded into 16-bit integers. Sealed chunks are compressed with Zstandard, achieving an audited **98.37% disk space reduction** compared to raw JSON.

### 4. SIMD Bitset Query Engine
Instead of disk-heavy inverted indexes, VortexLogs represents attribute matches as compressed bitsets. Multi-predicate queries (`service:auth AND level:ERROR`) resolve through hardware-accelerated bitwise operations, delivering **sub-millisecond queries across 10,000,000+ rows**.

### 5. Embedded Quantum Log Studio
No separate frontend services or external dependencies. A sleek, glassmorphic dark-mode web console with real-time SSE live tail is embedded directly inside the Go binary (`embed.FS`).

---

## 🚀 Running VortexLogs in 10 Seconds

```bash
docker run -d \
  -p 9428:9428 \
  -p 9429:9429/udp \
  -v vortex_data:/data \
  ianshugarg/vortexlogs:latest
```

Visit `http://localhost:9428` and stream logs instantly over HTTP or Syslog RFC 5424.

- **GitHub**: https://github.com/GargAnshu9468/vortexlogs
- **Docker Hub**: https://hub.docker.com/r/ianshugarg/vortexlogs
