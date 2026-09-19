package server

import (
	"testing"

	"github.com/GargAnshu9468/vortexlogs/internal/storage"
)

func TestParseSyslogEdgeCases(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected bool // true if entry != nil
	}{
		{"Empty string", "", false},
		{"Single word", "hello", true},
		{"Two words (previous panic trigger)", "service started", true},
		{"Three words", "service started successfully", true},
		{"Four words RFC3164 candidate", "Oct 11 22:14:15 host app: test message", true},
		{"RFC5424 timestamp format", "2026-09-19T12:00:00Z host app test message", true},
		{"With PRI <14>", "<14>hello", true},
		{"With PRI two words", "<14>hello world", true},
		{"With PRI invalid", "<99999>invalid pri format", true},
		{"Special characters and unicode", "🔥 Emergency shutdown in region us-east-1", true},
		{"Very long log string", string(make([]byte, 8000)), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("parseSyslog panicked on input %q: %v", tc.input, r)
				}
			}()

			entry := parseSyslog(tc.input, "127.0.0.1:514")
			if tc.expected && entry == nil {
				t.Errorf("expected entry for %q, got nil", tc.input)
			} else if !tc.expected && entry != nil {
				t.Errorf("expected nil entry for %q, got %+v", tc.input, entry)
			}
		})
	}
}

func TestParseSyslogSeverityMapping(t *testing.T) {
	tests := []struct {
		pri      string
		expected storage.Level
	}{
		{"<0>", storage.LevelError}, // Emergency
		{"<3>", storage.LevelError}, // Error
		{"<4>", storage.LevelWarn},  // Warning
		{"<6>", storage.LevelInfo},  // Info
		{"<7>", storage.LevelDebug}, // Debug
	}

	for _, tt := range tests {
		raw := tt.pri + "2026-09-19T12:00:00Z myhost myapp test message"
		entry := parseSyslog(raw, "127.0.0.1:514")
		if entry == nil {
			t.Fatalf("failed to parse %s", raw)
		}
		if entry.Level != tt.expected {
			t.Errorf("for PRI %s expected level %v, got %v", tt.pri, tt.expected, entry.Level)
		}
	}
}
