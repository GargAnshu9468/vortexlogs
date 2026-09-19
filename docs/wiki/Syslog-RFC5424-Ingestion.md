# Syslog RFC 5424 & RFC 3164 Ingestion

VortexLogs features an embedded, high-performance syslog daemon capable of listening concurrently over **UDP** and **TCP** (default port `:9429`).

---

## 📡 Forwarding Logs from rsyslog

Add the following to your `/etc/rsyslog.d/50-vortexlogs.conf`:

```text
# Forward all system logs to VortexLogs via UDP
*.* @127.0.0.1:9429;RSYSLOG_SyslogProtocol23Format

# Or forward via TCP for reliable delivery
*.* @@127.0.0.1:9429;RSYSLOG_SyslogProtocol23Format
```

Restart rsyslog:
```bash
sudo systemctl restart rsyslog
```

---

## ⚡ Forwarding Logs from Vector

Vector is an ultra-fast observability pipeline. In `vector.yaml`:

```yaml
sources:
  journald_logs:
    type: journald

transforms:
  format_vortex:
    type: remap
    inputs: ["journald_logs"]
    source: |
      .service = .service_name || "systemd"
      .level = .severity || "INFO"

sinks:
  to_vortexlogs:
    type: socket
    inputs: ["format_vortex"]
    address: 127.0.0.1:9429
    mode: udp
    encoding:
      codec: text
```

---

## 🛡️ Testing Syslog Manually via `nc` / `logger`

You can test ingestion instantly using standard CLI tools:

```bash
# Using standard Linux logger (RFC 3164 / 5424)
logger -d -n 127.0.0.1 -P 9429 -t auth-service "User authentication failed for admin"

# Using netcat directly
echo "<14>1 2026-09-19T12:00:00.000Z edge-01 gateway 1234 - - [vortex level=ERROR] Out of database connections" | nc -u 127.0.0.1 9429
```
Logs will immediately stream into the storage engine and appear in **Quantum Log Studio** in real time!
