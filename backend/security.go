package backend

import (
	"net/http"
	"strings"

	"go.hasen.dev/vbeam"
)

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
