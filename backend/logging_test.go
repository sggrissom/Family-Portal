package backend

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLogStructured(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	t.Run("Basic log entry", func(t *testing.T) {
		buf.Reset()
		logStructured(logLevelInfo, logCategoryAuth, "test message", nil, nil)

		output := buf.String()
		if !strings.Contains(output, "test message") {
			t.Errorf("Expected log to contain 'test message', got: %s", output)
		}

		if !strings.Contains(output, "INFO") {
			t.Errorf("Expected log to contain 'INFO', got: %s", output)
		}

		if !strings.Contains(output, "AUTH") {
			t.Errorf("Expected log to contain 'AUTH', got: %s", output)
		}
	})

	t.Run("Log entry with data", func(t *testing.T) {
		buf.Reset()
		testData := map[string]string{"key": "value"}
		logStructured(logLevelError, logCategorySystem, "error message", testData, nil)

		output := buf.String()
		if !strings.Contains(output, "error message") {
			t.Errorf("Expected log to contain 'error message', got: %s", output)
		}

		if !strings.Contains(output, "ERROR") {
			t.Errorf("Expected log to contain 'ERROR', got: %s", output)
		}

		if !strings.Contains(output, "SYSTEM") {
			t.Errorf("Expected log to contain 'SYSTEM', got: %s", output)
		}

		if !strings.Contains(output, "key") {
			t.Errorf("Expected log to contain data key, got: %s", output)
		}
	})

	t.Run("Log entry with request context", func(t *testing.T) {
		buf.Reset()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.RemoteAddr = "192.168.1.1:1234"

		logStructured(logLevelWarn, logCategoryAPI, "api warning", nil, req)

		output := buf.String()
		if !strings.Contains(output, "api warning") {
			t.Errorf("Expected log to contain 'api warning', got: %s", output)
		}

		if !strings.Contains(output, "WARN") {
			t.Errorf("Expected log to contain 'WARN', got: %s", output)
		}

		if !strings.Contains(output, "API") {
			t.Errorf("Expected log to contain 'API', got: %s", output)
		}

		if !strings.Contains(output, "test-agent") {
			t.Errorf("Expected log to contain user agent, got: %s", output)
		}
	})
}

func TestLogInfo(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	LogInfo(LogCategoryAuth, "test info message")

	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Errorf("Expected log to contain 'test info message', got: %s", output)
	}

	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log to contain 'INFO', got: %s", output)
	}
}

func TestLogInfoWithRequest(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest("POST", "/api/test", nil)
	LogInfoWithRequest(req, LogCategoryAPI, "request info")

	output := buf.String()
	if !strings.Contains(output, "request info") {
		t.Errorf("Expected log to contain 'request info', got: %s", output)
	}

	if !strings.Contains(output, "INFO") {
		t.Errorf("Expected log to contain 'INFO', got: %s", output)
	}

	if !strings.Contains(output, "API") {
		t.Errorf("Expected log to contain 'API', got: %s", output)
	}
}

func TestLogWarn(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	testData := map[string]int{"count": 42}
	LogWarn(LogCategoryPhoto, "warning message", testData)

	output := buf.String()
	if !strings.Contains(output, "warning message") {
		t.Errorf("Expected log to contain 'warning message', got: %s", output)
	}

	if !strings.Contains(output, "WARN") {
		t.Errorf("Expected log to contain 'WARN', got: %s", output)
	}

	if !strings.Contains(output, "PHOTO") {
		t.Errorf("Expected log to contain 'PHOTO', got: %s", output)
	}

	if !strings.Contains(output, "42") {
		t.Errorf("Expected log to contain data value, got: %s", output)
	}
}

func TestLogErrorSimple(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	LogErrorSimple(LogCategorySystem, "error occurred")

	output := buf.String()
	if !strings.Contains(output, "error occurred") {
		t.Errorf("Expected log to contain 'error occurred', got: %s", output)
	}

	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected log to contain 'ERROR', got: %s", output)
	}

	if !strings.Contains(output, "SYSTEM") {
		t.Errorf("Expected log to contain 'SYSTEM', got: %s", output)
	}
}

func TestLogErrorWithRequest(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	req := httptest.NewRequest("DELETE", "/api/delete", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")

	LogErrorWithRequest(req, LogCategoryAdmin, "admin error")

	output := buf.String()
	if !strings.Contains(output, "admin error") {
		t.Errorf("Expected log to contain 'admin error', got: %s", output)
	}

	if !strings.Contains(output, "ERROR") {
		t.Errorf("Expected log to contain 'ERROR', got: %s", output)
	}

	if !strings.Contains(output, "ADMIN") {
		t.Errorf("Expected log to contain 'ADMIN', got: %s", output)
	}
}

func TestLogDebug(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	LogDebug(LogCategoryWorker, "debug message")

	output := buf.String()
	if !strings.Contains(output, "debug message") {
		t.Errorf("Expected log to contain 'debug message', got: %s", output)
	}

	if !strings.Contains(output, "DEBUG") {
		t.Errorf("Expected log to contain 'DEBUG', got: %s", output)
	}

	if !strings.Contains(output, "WORKER") {
		t.Errorf("Expected log to contain 'WORKER', got: %s", output)
	}
}

func TestGetClientIP(t *testing.T) {
	t.Run("X-Forwarded-For header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

		ip := getClientIP(req)
		if ip != "192.168.1.1" {
			t.Errorf("Expected IP '192.168.1.1', got '%s'", ip)
		}
	})

	t.Run("Single X-Forwarded-For", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1")

		ip := getClientIP(req)
		if ip != "203.0.113.1" {
			t.Errorf("Expected IP '203.0.113.1', got '%s'", ip)
		}
	})

	t.Run("X-Real-IP header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Real-IP", "198.51.100.1")

		ip := getClientIP(req)
		if ip != "198.51.100.1" {
			t.Errorf("Expected IP '198.51.100.1', got '%s'", ip)
		}
	})

	t.Run("RemoteAddr fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "127.0.0.1:8080"

		ip := getClientIP(req)
		if ip != "127.0.0.1:8080" {
			t.Errorf("Expected IP '127.0.0.1:8080', got '%s'", ip)
		}
	})

	t.Run("X-Forwarded-For takes precedence", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "172.16.0.1")
		req.Header.Set("X-Real-IP", "192.168.1.1")
		req.RemoteAddr = "127.0.0.1:8080"

		ip := getClientIP(req)
		if ip != "172.16.0.1" {
			t.Errorf("Expected IP '172.16.0.1', got '%s'", ip)
		}
	})
}

func TestLogEntryJSONStructure(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	testData := map[string]interface{}{
		"action": "test",
		"count":  123,
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-browser/1.0")
	req.RemoteAddr = "192.168.1.100:1234"

	logStructured(logLevelInfo, logCategoryAuth, "structured test", testData, req)

	output := strings.TrimSpace(buf.String())

	// Extract just the JSON part (after the timestamp prefix)
	// The format is "YYYY/MM/DD HH:MM:SS {json}"
	parts := strings.SplitN(output, " {", 2)
	if len(parts) != 2 {
		t.Fatalf("Expected log output to contain JSON, got: %s", output)
	}
	jsonPart := "{" + parts[1]

	// Parse the JSON to verify structure
	var entry logEntry
	err := json.Unmarshal([]byte(jsonPart), &entry)
	if err != nil {
		t.Errorf("Failed to parse log entry as JSON: %v", err)
		return
	}

	// Verify fields
	if entry.Level != logLevelInfo {
		t.Errorf("Expected level 'INFO', got '%s'", entry.Level)
	}

	if entry.Category != logCategoryAuth {
		t.Errorf("Expected category 'AUTH', got '%s'", entry.Category)
	}

	if entry.Message != "structured test" {
		t.Errorf("Expected message 'structured test', got '%s'", entry.Message)
	}

	if entry.IP == "" {
		t.Error("Expected IP to be set")
	}

	if entry.UserAgent != "test-browser/1.0" {
		t.Errorf("Expected UserAgent 'test-browser/1.0', got '%s'", entry.UserAgent)
	}

	// Verify timestamp is recent (within 1 second)
	if time.Since(entry.Timestamp) > time.Second {
		t.Error("Expected timestamp to be recent")
	}
}
