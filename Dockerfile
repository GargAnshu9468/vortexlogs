# Build stage with native compilation using build platform
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk update && apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

# Statically link using Go native cross-compilation
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o /vortexlogs ./cmd/vortexlogs
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build -ldflags="-s -w" -o /vortexlogs-cli ./cmd/vortexlogs-cli

# Scratch runtime stage
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /vortexlogs /vortexlogs
COPY --from=builder /vortexlogs-cli /vortexlogs-cli

# Default data directory
VOLUME ["/data"]

# Expose ports: 9428 (HTTP / Studio / REST / SSE), 9429 (Syslog UDP/TCP RFC 5424/3164)
EXPOSE 9428 9429/udp 9429/tcp

ENTRYPOINT ["/vortexlogs"]
CMD ["-http", ":9428", "-syslog", ":9429", "-data", "/data"]
