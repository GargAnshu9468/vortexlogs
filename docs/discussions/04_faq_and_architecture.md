# ❓ Frequently Asked Questions & Architectural Decisions

Got technical questions about the internals of VortexLogs? Check the FAQ below or leave your question in this thread.

### Q: Why didn't you use Apache Lucene or Elasticsearch?
**A:** Lucene creates massive inverted indexes that result in 3x–5x write amplification and require huge amounts of JVM heap memory with unpredictable garbage collection stalls. VortexLogs uses a columnar chunk structure with bitset indexing: bitwise operations evaluate millions of rows per millisecond using CPU registers, with zero JVM overhead.

### Q: How does crash recovery work without data loss?
**A:** Every write is committed to a sequential, append-only Write-Ahead Log (WAL) guarded by CRC32 checksums before ring buffer dispatch. If the process is forcefully killed (`kill -9`) or the host loses power, VortexLogs replays the unsealed WAL files during startup and reconstitutes all columnar chunks in memory.

### Q: Can VortexLogs replace my Grafana Loki or Elasticsearch cluster?
**A:** Yes! For log ingestion, real-time live tailing, and structured field search (by service, level, host, timestamp, and message substring), VortexLogs delivers up to 10x faster query speeds at a fraction of the compute and storage footprint.

### Q: How much RAM does it need?
**A:** Idle memory footprint is under 40 MB. In high-throughput production environments ingesting over 1,000,000 logs/second, VortexLogs comfortably operates within a 512 MB to 1 GB memory limit.
