# Production Deployment & Docker Guide

VortexLogs compiles to a static, self-contained single binary with zero runtime dependencies. It can be deployed in seconds via Docker, Kubernetes, systemd, or bare metal.

---

## 🐳 Docker Deployment

### 1. One-Line Launch

```bash
docker run -d \
  --name vortexlogs \
  -p 9428:9428 \
  -p 9429:9429/udp \
  -p 9429:9429/tcp \
  -v vortexlogs_data:/data \
  --restart unless-stopped \
  ianshugarg/vortexlogs:latest
```

Open your browser to `http://localhost:9428` to access the embedded **Quantum Log Studio**.

### 2. Docker Compose

```yaml
version: '3.8'

services:
  vortexlogs:
    image: ianshugarg/vortexlogs:latest
    container_name: vortexlogs
    restart: unless-stopped
    ports:
      - "9428:9428"        # HTTP API & Web Studio
      - "9429:9429/udp"    # Syslog UDP RFC 5424
      - "9429:9429/tcp"    # Syslog TCP RFC 5424
    volumes:
      - ./vortex_data:/data
    command:
      - "-http"
      - ":9428"
      - "-syslog"
      - ":9429"
      - "-data"
      - "/data"
    deploy:
      resources:
        limits:
          memory: 512M
          cpus: '2.0'
```

---

## ☸️ Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vortexlogs
  labels:
    app: vortexlogs
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vortexlogs
  template:
    metadata:
      labels:
        app: vortexlogs
    spec:
      containers:
      - name: vortexlogs
        image: ianshugarg/vortexlogs:v1.0.0
        ports:
        - containerPort: 9428
          name: http
        - containerPort: 9429
          protocol: UDP
          name: syslog-udp
        - containerPort: 9429
          protocol: TCP
          name: syslog-tcp
        volumeMounts:
        - name: data-volume
          mountPath: /data
        resources:
          requests:
            memory: "128Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "2000m"
      volumes:
      - name: data-volume
        persistentVolumeClaim:
          claimName: vortexlogs-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: vortexlogs-service
spec:
  selector:
    app: vortexlogs
  ports:
  - name: http
    port: 9428
    targetPort: 9428
  - name: syslog-udp
    port: 9429
    protocol: UDP
    targetPort: 9429
  - name: syslog-tcp
    port: 9429
    protocol: TCP
    targetPort: 9429
```

---

## 🐧 Linux systemd Service

1. Copy the binary to `/usr/local/bin/`:
```bash
sudo cp bin/vortexlogs /usr/local/bin/
sudo chmod +x /usr/local/bin/vortexlogs
```

2. Create `/etc/systemd/system/vortexlogs.service`:
```ini
[Unit]
Description=VortexLogs Ultra-Fast Columnar Log Engine
After=network.target

[Service]
Type=simple
User=vortexlogs
Group=vortexlogs
ExecStart=/usr/local/bin/vortexlogs -http :9428 -syslog :9429 -data /var/lib/vortexlogs
Restart=always
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

3. Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable --now vortexlogs
sudo systemctl status vortexlogs
```
