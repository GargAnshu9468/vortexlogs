# Welcome to the VortexLogs Wiki

**VortexLogs** is an ultra-fast, single-binary columnar log aggregation and real-time observability engine engineered in pure Go. It delivers sub-millisecond query latencies across tens of millions of structured log events with zero JVM overhead, zero external dependencies, and up to 85% Zstandard columnar compression.

---

## ⚡ Quick Navigation

| Topic | Description |
| :--- | :--- |
| [Architecture Deep Dive](Architecture-Deep-Dive) | Lock-free LMAX Disruptor ring buffer, columnar chunk layout, SIMD bitset index, and append-only WAL |
| [Benchmarks and Performance](Benchmarks-and-Performance) | 1,200,000+ logs/sec ingestion, sub-millisecond bitset queries, and memory footprints |
| [Production Deployment & Docker](Production-Deployment-and-Docker) | Multi-arch Docker, systemd, Kubernetes, volume mounts, and memory limit tuning |
| [Quantum Log Studio](Quantum-Log-Studio) | Zero-dependency embedded web interface with real-time SSE live tail and dark glassmorphic UI |
| [Syslog RFC 5424 Ingestion](Syslog-RFC5424-Ingestion) | Universal syslog ingestion via dual UDP/TCP listeners for rsyslog, syslog-ng, and Vector |

---

## 🚀 Key Specifications

- **Ingestion Velocity**: 1,200,000+ log lines / second on commodity hardware
- **Query Performance**: Sub-millisecond indexed queries across 10M+ rows via roaring bitset intersections
- **Compression**: 80–85% storage reduction via Zstandard frame compression on sealed columnar blocks
- **Zero Dependencies**: Self-contained single binary with embedded web studio and dual UDP/TCP syslog listeners
- **Memory Footprint**: < 35 MB idle, predictable bounded memory under maximum load via lock-free ring buffering
- **Storage Resilience**: Zero data loss via CRC32-checksummed write-ahead log (WAL) with instant startup recovery
