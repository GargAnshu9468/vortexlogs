# 📊 Benchmarks Showcase & Community Hardware Results

Share your benchmark numbers running VortexLogs across your infrastructure!

To run the standard reproducible benchmark suite locally:

```bash
git clone https://github.com/GargAnshu9468/vortexlogs.git
cd vortexlogs
make bench
```

## Baseline Numbers (Apple M-Series / AMD EPYC NVMe)

- **Ingestion Velocity**: 2,312,887 logs/sec (Native Binary API), 1,271,812 logs/sec (HTTP REST), 1,480,000 logs/sec (Syslog UDP)
- **Point Query Latency**: 0.65 ms across 10,000,000 log records
- **Compression Ratio**: 85.4% – 98.3% disk space reduction vs raw JSON (Zstandard columnar)
- **Idle Memory**: ~38 MB RSS

Please share:
1. CPU & RAM specs
2. Storage type (NVMe SSD, EBS, SATA)
3. Ingest rate & latency observed
