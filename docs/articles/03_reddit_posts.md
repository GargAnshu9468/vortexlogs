# Reddit Post Templates: VortexLogs Launch

## Subreddit: r/golang

**Title:** I built VortexLogs: an ultra-fast columnar log engine in pure Go (2.31M logs/sec, lock-free ring buffer, embedded web studio)

**Post:**
Hey r/golang!

I wanted to share an open-source project I've been working on: **VortexLogs** (https://github.com/GargAnshu9468/vortexlogs).

It's an ultra-fast, single-binary columnar log aggregation engine designed as a lightweight, high-velocity alternative to bulky JVM-based log stacks.

### Key Go Architecture Highlights:
- **Lock-Free Disruptor Ring**: Built using `sync/atomic` with cache-line-padded 64-byte sequence pointers to eliminate false sharing. Handles **2,312,887 logs/sec** via Native Binary Protocol and **1,271,812 logs/sec** via HTTP REST without lock contention.
- **Append-Only WAL**: Sequential disk writes protected with CRC32 checksums delivering **1,050,000 writes/sec** for zero data loss and instant crash recovery.
- **Columnar ZSTD Chunks**: Compresses logs by **98.37%** using dictionary encoding and Zstandard frame compression.
- **SIMD Bitset Query Engine**: Boolean query filters (`AND`/`OR`) execute over compressed roaring bitsets in < 1 ms across 10M rows.
- **Embedded Web Studio**: Embeds the full HTML/CSS/JS frontend via `embed.FS`, featuring real-time SSE live tailing and engine telemetry.
- **Syslog RFC 5424 / 3164**: Dual UDP/TCP listeners for forwarding directly from rsyslog, Vector, or Fluentbit.

Single static binary: ~11 MB scratch Docker container.

Repo: https://github.com/GargAnshu9468/vortexlogs
Docker: `docker pull ianshugarg/vortexlogs:latest`

Check out `make bench` to run the benchmark suite locally. Would appreciate your feedback on the architecture!

---

## Subreddit: r/devops / r/selfhosted

**Title:** Sick of heavy JVM logging stacks? VortexLogs is an 11MB single-binary log engine with 2.31M logs/sec ingest and built-in web studio

**Post:**
If you've ever had your logging stack consume more RAM than the actual applications it's monitoring, VortexLogs is for you.

- 2.31M+ logs/sec Native Binary ingest (1.27M+ logs/sec HTTP REST)
- 1.05M writes/sec Durable WAL disk commits
- Sub-millisecond queries across 10M+ rows
- 98.37% storage savings via Zstandard columnar compression
- Embedded Quantum Log Studio (live tail + visual query builder)
- Dual UDP/TCP Syslog listener (RFC 5424 / 3164)
- Zero dependencies, ~11MB Docker image

One line to run:
`docker run -d -p 9428:9428 -p 9429:9429/udp -v vortex_data:/data ianshugarg/vortexlogs:latest`

GitHub: https://github.com/GargAnshu9468/vortexlogs
Docker Hub: https://hub.docker.com/r/ianshugarg/vortexlogs
