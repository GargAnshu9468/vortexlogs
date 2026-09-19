# 📊 Benchmarks Showcase & Community Hardware Results

Share your benchmark numbers running VortexLogs across your infrastructure!

To run the standard reproducible benchmark suite locally:

```bash
git clone https://github.com/GargAnshu9468/vortexlogs.git
cd vortexlogs
make bench
```

## Baseline Numbers (Apple M-Series / AMD EPYC NVMe)

- **Ingestion Velocity**: 1,240,000 logs/sec (HTTP API), 1,480,000 logs/sec (Syslog UDP)
- **Point Query Latency**: 0.65 ms across 10,000,000 log records
- **Compression Ratio**: 85.4% disk space reduction vs raw JSON (Zstandard columnar)
- **Idle Memory**: ~38 MB RSS

Please share:
1. CPU & RAM specs
2. Storage type (NVMe SSD, EBS, SATA)
3. Ingest rate & latency observed
