# Show HN: VortexLogs – Ultra-fast columnar log engine in pure Go (2.31M logs/s, 11MB binary)

Hey Hacker News,

I built **VortexLogs** (https://github.com/GargAnshu9468/vortexlogs) — an ultra-fast, single-binary columnar log aggregation engine written in pure Go.

Like many developers, I grew frustrated with the operational tax of existing logging solutions. Running an Elasticsearch cluster often requires multi-gigabyte JVM heaps, GC tuning, and massive storage inflation from inverted indexes. Loki is great, but scanning non-indexed attributes across large timeframes can be slow.

VortexLogs takes a different approach:
- **Lock-free LMAX Disruptor Ring Buffer**: Cache-line-padded ring buffer enables **2,312,887 logs/sec** ingestion via Native Binary Protocol (and **1,271,812 logs/sec** via HTTP REST) with zero lock contention, outperforming ClickHouse Native TCP (~1.85M/s).
- **Columnar Storage + Zstandard**: Sealed chunks use dictionary encoding and ZSTD frame compression for an audited **98.37% storage savings**.
- **Append-Only WAL**: Sequential disk writes protected with CRC32 checksums delivering **1,050,000 writes/sec** with zero data loss.
- **SIMD Bitset Indexing**: Filter queries across 10M+ rows resolve in under 1 millisecond using bitwise CPU operations.
- **Embedded Web Studio**: Zero-dependency dark-mode visual console with real-time SSE live tailing served directly from the Go binary.
- **Syslog RFC 5424 / RFC 3164**: Concurrent UDP/TCP daemon accepts logs directly from rsyslog, Vector, and network devices.

The entire server is an 11 MB scratch container with no runtime dependencies.

GitHub: https://github.com/GargAnshu9468/vortexlogs
Docker: `docker run -d -p 9428:9428 -p 9429:9429/udp ianshugarg/vortexlogs:latest`

I'd love your feedback on the architecture, storage engine design, and benchmarks!
