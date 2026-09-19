package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

// SyslogServer listens for incoming RFC 3164 & RFC 5424 syslog packets over UDP and TCP.
type SyslogServer struct {
	addr    string
	engine  *engine.Engine
	udpConn *net.UDPConn
	tcpList net.Listener
	mu      sync.Mutex
	closed  bool
	wg      sync.WaitGroup
}

// NewSyslogServer initializes a new dual UDP/TCP Syslog server.
func NewSyslogServer(addr string, eng *engine.Engine) *SyslogServer {
	return &SyslogServer{
		addr:   addr,
		engine: eng,
	}
}

// Start begins listening on both UDP and TCP at the configured address.
func (s *SyslogServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr error: %w", err)
	}
	uConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp error: %w", err)
	}
	s.udpConn = uConn

	// TCP listener
	tcpList, err := net.Listen("tcp", s.addr)
	if err != nil {
		_ = s.udpConn.Close()
		return fmt.Errorf("listen tcp error: %w", err)
	}
	s.tcpList = tcpList

	s.wg.Add(2)
	go s.listenUDP()
	go s.listenTCP()

	log.Printf("[VortexLogs Syslog] Ingestion active on UDP & TCP %s", s.addr)
	return nil
}

func (s *SyslogServer) listenUDP() {
	defer s.wg.Done()
	buf := make([]byte, 65535)

	for {
		n, remoteAddr, err := s.udpConn.ReadFrom(buf)
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}

		raw := string(buf[:n])
		entry := parseSyslog(raw, remoteAddr.String())
		if entry != nil {
			_ = s.engine.Ingest(entry)
		}
	}
}

func (s *SyslogServer) listenTCP() {
	defer s.wg.Done()

	for {
		conn, err := s.tcpList.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			defer c.Close()

			reader := bufio.NewReader(c)
			remoteAddr := c.RemoteAddr().String()

			for {
				line, err := reader.ReadString('\n')
				if len(line) > 0 {
					trimmed := strings.TrimRight(line, "\r\n")
					if len(trimmed) > 0 {
						entry := parseSyslog(trimmed, remoteAddr)
						if entry != nil {
							_ = s.engine.Ingest(entry)
						}
					}
				}
				if err != nil {
					if err != io.EOF {
						// Socket dropped or closed
					}
					return
				}
			}
		}(conn)
	}
}

// Stop gracefully terminates syslog listeners.
func (s *SyslogServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
	if s.tcpList != nil {
		_ = s.tcpList.Close()
	}
	s.mu.Unlock()

	c := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(c)
	}()

	select {
	case <-c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// parseSyslog parses RFC 3164 / RFC 5424 raw syslog frames.
func parseSyslog(raw string, source string) *storage.Entry {
	if len(raw) == 0 {
		return nil
	}

	raw = strings.TrimSpace(raw)
	labels := make(map[string]string)
	labels["source"] = "syslog"
	labels["sender"] = source

	level := storage.LevelInfo
	service := "system"
	message := raw
	ts := time.Now().UnixNano()

	// Check for <PRI> prefix e.g. <34>1 2023-10-11T...
	if strings.HasPrefix(raw, "<") {
		priEnd := strings.IndexByte(raw, '>')
		if priEnd > 1 && priEnd < 6 {
			priStr := raw[1:priEnd]
			if priVal, err := strconv.Atoi(priStr); err == nil {
				severity := priVal & 0x07
				facility := priVal >> 3
				labels["facility"] = strconv.Itoa(facility)

				switch severity {
				case 0, 1, 2, 3: // Emergency, Alert, Critical, Error
					level = storage.LevelError
				case 4: // Warning
					level = storage.LevelWarn
				case 5, 6: // Notice, Informational
					level = storage.LevelInfo
				case 7: // Debug
					level = storage.LevelDebug
				}
			}
			raw = strings.TrimSpace(raw[priEnd+1:])
		}
	}

	// Inspect RFC 5424 format: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID SD MSG
	// or RFC 3164 format: <PRI>TIMESTAMP HOSTNAME TAG: MSG
	parts := strings.Fields(raw)
	if len(parts) >= 2 {
		// Attempt RFC 3164 parsing
		if parsedTime, err := time.Parse("Jan 2 15:04:05", strings.Join(parts[0:3], " ")); err == nil && len(parts) >= 4 {
			now := time.Now()
			parsedTime = parsedTime.AddDate(now.Year(), 0, 0)
			ts = parsedTime.UnixNano()
			labels["host"] = parts[3]

			rest := strings.Join(parts[4:], " ")
			if colonIdx := strings.IndexByte(rest, ':'); colonIdx != -1 {
				service = strings.TrimSpace(rest[:colonIdx])
				message = strings.TrimSpace(rest[colonIdx+1:])
			} else {
				message = rest
			}
		} else if parsedRFC3339, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
			ts = parsedRFC3339.UnixNano()
			if len(parts) > 1 {
				labels["host"] = parts[1]
			}
			if len(parts) > 2 {
				service = parts[2]
			}
			if len(parts) > 3 {
				message = strings.Join(parts[3:], " ")
			}
		} else {
			message = raw
		}
	} else {
		message = raw
	}

	return &storage.Entry{
		Timestamp: ts,
		Level:     level,
		Service:   service,
		Message:   message,
		Labels:    labels,
	}
}
