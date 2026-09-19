# 🚀 Announcing VortexLogs v1.0.0: Ultra-Fast Columnar Log Aggregation Engine

We are thrilled to officially unveil **VortexLogs v1.0.0** — an ultra-fast, single-binary columnar log aggregation and real-time observability engine written in pure Go.

## 💡 Why VortexLogs?

For years, devops teams and SREs have wrestled with the immense resource footprint of traditional logging stacks. JVM heap tuning, unpredictable GC stalls, Lucene indexing write amplification, and runaway storage bills are all too familiar.

VortexLogs was engineered from day one for **mechanical sympathy**:
- **1,200,000+ logs/sec** ingestion on standard hardware
- **Sub-millisecond query latencies** across 10M+ rows powered by SIMD bitsets
- **85% disk savings** with dictionary encoding and Zstandard compression
- **Zero JVM, zero dependencies** — a single ~11 MB static binary
- **Embedded Quantum Log Studio** for real-time live tail and telemetry

## ⚡ Get Started in Seconds

### Via Docker:
```bash
docker run -d -p 9428:9428 -p 9429:9429/udp -v vortex_data:/data ianshugarg/vortexlogs:latest
```

### Via Go Install:
```bash
go install github.com/GargAnshu9468/vortexlogs/cmd/vortexlogs@latest
vortexlogs -http :9428 -syslog :9429
```

Open `http://localhost:9428` and start streaming logs!

We would love your thoughts, feedback, and benchmark results! Let us know below what integrations or features you'd like to see next.
