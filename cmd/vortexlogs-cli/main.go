package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorPink   = "\033[35m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
)

func levelColor(lvl string) string {
	switch strings.ToUpper(lvl) {
	case "DEBUG":
		return colorCyan
	case "INFO":
		return colorGreen
	case "WARN", "WARNING":
		return colorYellow
	case "ERROR":
		return colorRed
	case "FATAL", "CRITICAL":
		return colorPink
	default:
		return colorReset
	}
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "query":
		runQuery(os.Args[2:])
	case "tail":
		runTail(os.Args[2:])
	case "stats":
		runStats(os.Args[2:])
	case "ingest":
		runIngest(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`VortexLogs CLI — Ultra-Fast Log Engine Client

Usage:
  vortexlogs-cli <command> [flags]

Commands:
  query     Execute a high-speed bitset query against VortexLogs
  tail      Follow live log stream with colorized terminal output
  stats     Display live engine telemetry and compression ratios
  ingest    Pipe NDJSON or raw log lines to VortexLogs ingestion API

Examples:
  vortexlogs-cli tail -server localhost:9428
  vortexlogs-cli query -search "timeout" -level ERROR
  cat app.log | vortexlogs-cli ingest -service payment-service`)
}

func runTail(args []string) {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	serverHost := fs.String("server", "localhost:9428", "VortexLogs server host:port")
	filter := fs.String("filter", "", "Search substring filter")
	_ = fs.Parse(args)

	u := url.URL{Scheme: "ws", Host: *serverHost, Path: "/api/v1/tail"}
	if *filter != "" {
		q := u.Query()
		q.Set("filter", *filter)
		u.RawQuery = q.Encode()
	}

	fmt.Printf("%s[VortexLogs CLI]%s Connecting to live stream at %s%s%s...\n", colorCyan, colorReset, colorBold, u.String(), colorReset)

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		fmt.Printf("%sFailed to connect to %s: %v%s\n", colorRed, u.String(), err, colorReset)
		os.Exit(1)
	}
	defer c.Close()

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			fmt.Printf("\n%sStream disconnected: %v%s\n", colorYellow, err, colorReset)
			return
		}

		var entry struct {
			Timestamp int64             `json:"timestamp"`
			Level     string            `json:"level"`
			Service   string            `json:"service"`
			Message   string            `json:"message"`
			Labels    map[string]string `json:"labels"`
		}

		if err := json.Unmarshal(message, &entry); err == nil {
			t := time.Unix(0, entry.Timestamp).Format("15:04:05.000")
			lvlCol := levelColor(entry.Level)
			fmt.Printf("%s%s%s %s[%-5s]%s %s%-16s%s %s\n",
				colorDim, t, colorReset,
				lvlCol, entry.Level, colorReset,
				colorPink, entry.Service, colorReset,
				entry.Message,
			)
		} else {
			fmt.Println(string(message))
		}
	}
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	serverHost := fs.String("server", "localhost:9428", "VortexLogs server host:port")
	search := fs.String("search", "", "Search filter")
	level := fs.String("level", "", "Log level filter (DEBUG, INFO, WARN, ERROR)")
	service := fs.String("service", "", "Service name filter")
	limit := fs.Int("limit", 50, "Max entries to fetch")
	_ = fs.Parse(args)

	reqBody := map[string]any{
		"search": *search,
		"limit":  *limit,
	}
	if *level != "" {
		reqBody["level"] = *level
	}
	if *service != "" {
		reqBody["labels"] = map[string]string{"service": *service}
	}

	payload, _ := json.Marshal(reqBody)
	endpoint := fmt.Sprintf("http://%s/api/v1/query", *serverHost)
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(payload))
	if err != nil {
		fmt.Printf("%sQuery request failed: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		TotalHits  int     `json:"total_hits"`
		DurationMs float64 `json:"duration_ms"`
		Entries    []struct {
			Timestamp int64  `json:"timestamp"`
			Level     string `json:"level"`
			Service   string `json:"service"`
			Message   string `json:"message"`
		} `json:"entries"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Printf("Response: %s\n", string(body))
		return
	}

	fmt.Printf("%s[VortexLogs Query]%s Found %s%d%s records in %s%.3f ms%s\n\n",
		colorCyan, colorReset, colorBold, result.TotalHits, colorReset, colorGreen, result.DurationMs, colorReset)

	for _, e := range result.Entries {
		t := time.Unix(0, e.Timestamp).Format("2006-01-02 15:04:05.000")
		lvlCol := levelColor(e.Level)
		fmt.Printf("%s%s%s %s[%-5s]%s %s%-16s%s %s\n",
			colorDim, t, colorReset,
			lvlCol, e.Level, colorReset,
			colorPink, e.Service, colorReset,
			e.Message,
		)
	}
}

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	serverHost := fs.String("server", "localhost:9428", "VortexLogs server host:port")
	_ = fs.Parse(args)

	endpoint := fmt.Sprintf("http://%s/api/v1/stats", *serverHost)
	resp, err := http.Get(endpoint)
	if err != nil {
		fmt.Printf("%sFailed to fetch stats: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var formatted bytes.Buffer
	_ = json.Indent(&formatted, body, "", "  ")

	fmt.Printf("%s--- VortexLogs Telemetry ---%s\n%s\n", colorCyan, colorReset, formatted.String())
}

func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	serverHost := fs.String("server", "localhost:9428", "VortexLogs server host:port")
	service := fs.String("service", "cli-client", "Service tag to apply to ingested lines")
	_ = fs.Parse(args)

	scanner := bufio.NewScanner(os.Stdin)
	var batch []map[string]any
	count := 0

	flush := func() {
		if len(batch) == 0 {
			return
		}
		data, _ := json.Marshal(batch)
		endpoint := fmt.Sprintf("http://%s/api/v1/ingest", *serverHost)
		resp, err := http.Post(endpoint, "application/json", bytes.NewReader(data))
		if err == nil {
			_ = resp.Body.Close()
		}
		count += len(batch)
		batch = batch[:0]
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		batch = append(batch, map[string]any{
			"timestamp": time.Now().UnixNano(),
			"level":     "INFO",
			"service":   *service,
			"message":   line,
		})

		if len(batch) >= 100 {
			flush()
		}
	}
	flush()

	fmt.Printf("%s[VortexLogs Ingest]%s Successfully piped %d log lines.\n", colorGreen, colorReset, count)
}
