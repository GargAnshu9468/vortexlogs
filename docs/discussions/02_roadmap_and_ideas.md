# 🗺️ VortexLogs Roadmap & Community Feature Ideas

Welcome to the feature ideation and roadmap forum for VortexLogs. Here are the core focus areas currently planned:

## Planned Milestones

### v1.1.0 — Ecosystem Ingestion & Forwarding
- [ ] Native OpenTelemetry (OTLP/gRPC) receiver endpoint on `:4317`
- [ ] Prometheus metrics exporter (`/metrics` standard endpoint)
- [ ] LogQL query syntax subset compatibility for drop-in Grafana Loki replacement
- [ ] Automated log retention policies and chunk expiration daemon

### v1.2.0 — Distributed Clustering & Tiering
- [ ] Raft-based multi-node metadata consensus
- [ ] Hot/Cold tiering (local NVMe hot storage -> S3 / GCS object storage cold chunks)
- [ ] Distributed scatter-gather query execution across nodes

### v2.0.0 — Native Anomaly Detection
- [ ] Stream-level statistical anomaly detection (z-score and rolling quantile anomaly alerts)
- [ ] Webhook alerting integrations (Slack, PagerDuty, Discord)

---

What capabilities would make VortexLogs most impactful for your infrastructure? Drop your feedback, feature requests, and architecture ideas below!
