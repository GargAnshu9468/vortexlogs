# Quantum Log Studio

VortexLogs embeds **Quantum Log Studio** directly into the server binary using Go's `embed` package (`embed.FS`). There are no Node.js dependencies, no build steps, and no separate frontend servers required.

---

## 🌟 Key Features

1. **Cybernetic Dark Glassmorphic Interface**:
   - Deep obsidian backgrounds (`#07090e`), neon cyan (`#00f0ff`), electric violet (`#a855f7`), and crisp typography.
   - Designed for low-fatigue NOC and SRE operational monitoring.

2. **Real-Time Live Tail**:
   - Powered by Server-Sent Events (SSE) and WebSockets on `/api/v1/tail`.
   - Streams incoming logs to the browser with zero polling and sub-10ms UI render latency.

3. **Bitset Query Builder**:
   - Filter logs dynamically by `service`, `level`, `host`, or full-text substring.
   - Visual execution time badges display microsecond/millisecond query durations.

4. **Engine Diagnostics Telemetry**:
   - Real-time gauges tracking:
     - Ingestion throughput (logs/sec)
     - Active vs. sealed chunk counts
     - Columnar Zstandard compression ratio
     - Ring buffer capacity utilization

---

## 🖥️ Accessing the Studio

Whenever VortexLogs is running:

```bash
# Launch server
./bin/vortexlogs -http :9428 -syslog :9429 -data ./data
```

Navigate to:
```
http://localhost:9428
```
All static assets (HTML, CSS, JS, SVG assets) are compiled into the binary and served with instantaneous in-memory performance.
