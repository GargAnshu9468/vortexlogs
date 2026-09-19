package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/GargAnshu9468/vortexlogs/internal/engine"
	"github.com/GargAnshu9468/vortexlogs/internal/server"
	"github.com/GargAnshu9468/vortexlogs/web"
)

const banner = `
  _    __           __             __                   
 | |  / /___  _____/ /____  _  __/ /   ____  ____ ______
 | | / / __ \/ ___/ __/ _ \| |/_/ /   / __ \/ __ '/ ___/ 
 | |/ / /_/ / /  / /_/  __/>  </ /___/ /_/ / /_/ (__  )  
 |___/\____/_/   \__/\___/_/|_/_____/\____/\__, /____/   
                                          /____/         
  [VortexLogs] Ultra-Fast Columnar Log Aggregation & Observability Engine
  Pure Go • Lock-Free Ring Buffer • Bitset Index • ZSTD Compression
`

func main() {
	httpAddr := flag.String("http", ":9428", "HTTP API and Web Studio listen address")
	syslogAddr := flag.String("syslog", ":9429", "Dual UDP & TCP Syslog RFC 5424 listen address")
	dataDir := flag.String("data", "./data", "Directory for WAL and columnar chunks storage")
	flag.Parse()

	fmt.Print(banner)
	fmt.Printf("\n[VortexLogs] Initializing storage engine in: %s\n", *dataDir)

	eng, err := engine.Open(*dataDir)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize VortexLogs engine: %v", err)
	}

	// 1. Start HTTP Server & Embedded Studio UI
	httpServer := server.NewHTTPServer(*httpAddr, eng, web.GetStudioFS())
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != context.Canceled {
			log.Printf("[VortexLogs HTTP] Server stopped: %v", err)
		}
	}()

	// 2. Start Dual UDP/TCP Syslog Ingestion Server
	syslogServer := server.NewSyslogServer(*syslogAddr, eng)
	if err := syslogServer.Start(); err != nil {
		log.Printf("[WARN] Failed to start Syslog server on %s: %v", *syslogAddr, err)
	}

	fmt.Printf("[VortexLogs] Ready! Quantum Log Studio accessible at http://localhost%s\n", *httpAddr)
	fmt.Printf("[VortexLogs] Syslog RFC 5424 ingestion listening on port %s (UDP/TCP)\n", *syslogAddr)
	fmt.Printf("[VortexLogs] Ingest API endpoint: POST http://localhost%s/api/v1/ingest\n\n", *httpAddr)

	// Graceful shutdown handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n[VortexLogs] Initiating graceful shutdown...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = syslogServer.Stop(shutdownCtx)
	if err := eng.Close(); err != nil {
		log.Printf("[ERROR] Engine close error: %v", err)
	}

	fmt.Println("[VortexLogs] Shutdown completed safely. Bye!")
}
