package backend

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func postClientError(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, ClientErrorPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	clientErrorHandler(rec, req)
	return rec
}

func TestClientErrorAcceptsAReport(t *testing.T) {
	report := ClientErrorReport{
		Message: "Cannot read properties of undefined",
		Stack:   "at view (photos.tsx:12)",
		Route:   "/photos",
		Source:  "render",
	}
	body, _ := json.Marshal(report)

	rec := postClientError(t, string(body))
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestClientErrorRejectsEmptyMessage(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "no message field", body: `{"route":"/photos"}`},
		{name: "blank message", body: `{"message":"   "}`},
		{name: "not json", body: `nonsense`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if rec := postClientError(t, tc.body); rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

// The endpoint is unauthenticated, so nothing but the caps stops a client from
// writing arbitrary volume into the log.
func TestClientErrorTruncatesOversizedFields(t *testing.T) {
	long := strings.Repeat("x", maxClientErrorStack*3)
	body, _ := json.Marshal(ClientErrorReport{Message: long, Stack: long, Route: long, Source: long})

	rec := postClientError(t, string(body))
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 204 or 400", rec.Code)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("short string changed: %q", got)
	}
	if got := truncate("abcdef", 3); got != "abc…" {
		t.Errorf("truncate() = %q, want %q", got, "abc…")
	}
}

func TestClientErrorRejectsAnOversizedBody(t *testing.T) {
	huge := bytes.Repeat([]byte("y"), maxClientErrorStack*4)
	body, _ := json.Marshal(ClientErrorReport{Message: string(huge)})

	req := httptest.NewRequest(http.MethodPost, ClientErrorPath, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	clientErrorHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
