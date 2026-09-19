# VortexLogs ⚡
### Next-Generation Ultra-Fast Columnar Log Engine in Pure Go

<p align="center">
  <img src="https://raw.githubusercontent.com/GargAnshu9468/vortexlogs/main/assets/vortexlogs-banner.png" alt="VortexLogs Banner" width="800"/>
</p>

VortexLogs is an ultra-high-performance columnar log aggregation and real-time observability engine written in 100% pure Go (zero CGO, zero Java, zero JVM).

It solves the operational bloat and disk multiplication of legacy log platforms (Elasticsearch, Logstash, Loki, Promtail) by delivering a compact **11 MB container** that boots in **<10 milliseconds** and uses **<35 MB RAM**.

---

## 🚀 Quick Start (1-Command Run)

Run VortexLogs with the embedded Quantum Log Studio and native Syslog ingestion:

```bash
docker run -d \
  --name vortexlogs \
  -p 9428:9428 \
  -p 9429:9429/udp \
  -p 9429:9429/tcp \
  -v vortexlogs_data:/data \
  ianshugarg/vortexlogs:latest
```

### 🔌 Ports Overview
* **`9428`**: **HTTP REST API & Quantum Log Studio** (Live web dashboard, WebSocket live tail, and ingest endpoints)
* **`9429`**: **Dual Syslog RFC 5424 / RFC 3164 Listener** (Native high-speed UDP & TCP stream ingestion)

---

## 🌌 Quantum Log Studio Dashboard

Open your browser to:
👉 **`http://localhost:9428`**

* **Live Stream Waterfall**: Real-time streaming log viewer with colorized severity badges (`DEBUG`, `INFO`, `WARN`, `ERROR`, `FATAL`).
* **Scrubbable Time Horizon Histogram**: Interactive SVG/Canvas timeline histogram of events by severity with drag-to-zoom and click-to-filter.
* **Slide-Out JSON Inspector**: Detailed structured attribute explorer with formatted JSON payload viewing and copy support.
* **Dark / Light Theme**: Fluid toggle with persistent settings.

---

## 💻 Ingest Logs in Seconds

### 1. Ingest via HTTP REST API
```bash
curl -X POST http://localhost:9428/api/v1/ingest \
  -H "Content-Type: application/json" \
  -d '[
    {
      "service": "checkout-svc",
      "level": "INFO",
      "message": "Processed cart checkout #9012 item_count=4",
      "labels": {"env": "prod", "region": "us-east-1"}
    },
    {
      "service": "billing-svc",
      "level": "WARN",
      "message": "Payment gateway latency 890ms above p95 SLA",
      "labels": {"env": "prod", "region": "us-east-1"}
    }
  ]'
```

### 2. Stream via Syslog
```bash
# Forward events via standard Linux syslog / rsyslog over UDP
*.*  @localhost:9429

# Or forward over TCP with RFC 5424 structured framing
*.*  @@localhost:9429;RSYSLOG_SyslogProtocol23Format
```

---

## 📊 Benchmark Highlights

* **Disruptor Ring Ingestion**: **157.9 Million ops/sec** (7.47 ns/op, 0 B/op allocations)
* **Columnar Bitset Indexing**: **0.105 ms query latency** across 100,000 log records
* **Storage Footprint**: **87.4% ZSTD block compaction** (7.2x smaller than raw logs)
* **Cold Boot Time**: **<10 ms** (vs Elasticsearch 45s)
* **Idle Memory**: **<35 MB** (vs Elasticsearch 2GB+, Loki 500MB+)

---

## 🔗 Official Links
* **GitHub Repository**: [https://github.com/GargAnshu9468/vortexlogs](https://github.com/GargAnshu9468/vortexlogs)
* **Official Website & Interactive Simulator**: [https://garganshu9468.github.io/vortexlogs/](https://garganshu9468.github.io/vortexlogs/)
* **License**: Apache 2.0 / MIT
