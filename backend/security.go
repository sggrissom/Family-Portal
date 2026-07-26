package backend

import (
	"net/http"
	"strings"

	"go.hasen.dev/vbeam"
)

const (
	maxJSONRequestBytes   int64 = 1 << 20   // 1 MiB
	maxPhotoRequestBytes  int64 = 52 << 20  // 50 MiB file plus multipart metadata
	maxImportRequestBytes int64 = 512 << 20 // Full-family archives can contain photos.
)

// RequestSizeLimitWrapper applies endpoint-aware body limits before requests
// reach parsers. MaxBytesReader also protects requests that use chunked
// transfer encoding and therefore have no Content-Length header.
type RequestSizeLimitWrapper struct {
	next http.Handler
}

func NewRequestSizeLimitWrapper(next http.Handler) *RequestSizeLimitWrapper {
	return &RequestSizeLimitWrapper{next: next}
}

func requestBodyLimit(r *http.Request) int64 {
	switch r.URL.Path {
	case "/api/upload-photo":
		return maxPhotoRequestBytes
	case "/api/import-bundle":
		return maxImportRequestBytes
	}

	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return maxJSONRequestBytes
	}
	return 0
}

func (rw *RequestSizeLimitWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if limit := requestBodyLimit(r); limit > 0 {
		if r.ContentLength > limit {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}
	rw.next.ServeHTTP(w, r)
}

// SecurityWrapper wraps the vbeam.Application with security headers
type SecurityWrapper struct {
	app *vbeam.Application
}

// NewSecurityWrapper creates a new security wrapper around the vbeam application
func NewSecurityWrapper(app *vbeam.Application) *SecurityWrapper {
	return &SecurityWrapper{app: app}
}

func isWebSocketRequest(r *http.Request) bool {
	connHeader := strings.ToLower(r.Header.Get("Connection"))
	if !strings.Contains(connHeader, "upgrade") {
		return false
	}

	upgradeHeader := strings.TrimSpace(r.Header.Get("Upgrade"))
	if !strings.EqualFold(upgradeHeader, "websocket") {
		return false
	}

	return true
}

// ServeHTTP implements http.Handler and adds security headers to all responses
func (sw *SecurityWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle WebSocket requests for /ws/chat before adding any headers
	if isWebSocketRequest(r) && r.URL.Path == "/ws/chat" {
		HandleWebSocketChat(sw.app)(w, r)
		return
	}

	// Add security headers to non-WebSocket responses
	addSecurityHeaders(w)
	sw.app.ServeHTTP(w, r)
}

func addSecurityHeaders(w http.ResponseWriter) {
	// Security headers
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

	// Content Security Policy - restrictive but allows inline styles and WebSocket connections
	csp := "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"font-src 'self'; " +
		"connect-src 'self' ws: wss:; " +
		"frame-ancestors 'none';"
	w.Header().Set("Content-Security-Policy", csp)
}
