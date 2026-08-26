package backend

import (
	"encoding/json"
	"family/cfg"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimitRule struct {
	Name   string
	Burst  int
	Window time.Duration
}

func (r RateLimitRule) ratePerSecond() float64 {
	if r.Window <= 0 {
		return 0
	}
	return float64(r.Burst) / r.Window.Seconds()
}

var (
	rateRuleLogin         = RateLimitRule{Name: "login", Burst: 10, Window: 5 * time.Minute}
	rateRuleSignup        = RateLimitRule{Name: "signup", Burst: 5, Window: time.Hour}
	rateRulePasswordReset = RateLimitRule{Name: "password-reset", Burst: 5, Window: 15 * time.Minute}
	rateRuleInviteCode    = RateLimitRule{Name: "invite-code", Burst: 10, Window: 15 * time.Minute}
	rateRuleRefresh       = RateLimitRule{Name: "refresh", Burst: 30, Window: 5 * time.Minute}
	rateRuleAI            = RateLimitRule{Name: "ai", Burst: 10, Window: time.Hour}
	rateRuleImport        = RateLimitRule{Name: "import", Burst: 5, Window: time.Hour}
	rateRuleUpload        = RateLimitRule{Name: "upload", Burst: 120, Window: 10 * time.Minute}
	rateRuleWebSocket     = RateLimitRule{Name: "websocket", Burst: 30, Window: 5 * time.Minute}
	rateRulePhotoRead     = RateLimitRule{Name: "photo-read", Burst: 600, Window: 5 * time.Minute}
	rateRuleSnapshot      = RateLimitRule{Name: "snapshot", Burst: 10, Window: time.Hour}
	rateRuleDefault       = RateLimitRule{Name: "default", Burst: 300, Window: time.Minute}
)

var exactPathRules = map[string]RateLimitRule{
	"/api/login":              rateRuleLogin,
	"/api/login/google":       rateRuleLogin,
	"/api/login/google/token": rateRuleLogin,
	"/api/google/callback":    rateRuleLogin,
	"/api/refresh":            rateRuleRefresh,
	"/api/change-password":    rateRuleLogin,
	"/api/delete-account":     rateRuleLogin,
	"/api/upload-photo":       rateRuleUpload,
	"/api/import-bundle":      rateRuleImport,
	"/ws/chat":                rateRuleWebSocket,
	SnapshotPath:              rateRuleSnapshot,

	"/rpc/CreateAccount":              rateRuleSignup,
	"/rpc/RequestPasswordReset":       rateRulePasswordReset,
	"/rpc/ValidatePasswordResetToken": rateRulePasswordReset,
	"/rpc/ResetPassword":              rateRulePasswordReset,
	"/rpc/JoinFamily":                 rateRuleInviteCode,
	"/rpc/AcceptFamilyLink":           rateRuleInviteCode,
	"/rpc/ProcessAIImport":            rateRuleAI,
	"/rpc/ListAIModels":               rateRuleAI,
	"/rpc/ImportData":                 rateRuleImport,
}

var prefixPathRules = []struct {
	prefix string
	rule   RateLimitRule
}{
	{prefix: "/api/photo/", rule: rateRulePhotoRead},
}

func ruleForPath(path string) (RateLimitRule, bool) {
	if rule, ok := exactPathRules[path]; ok {
		return rule, true
	}
	for _, entry := range prefixPathRules {
		if strings.HasPrefix(path, entry.prefix) {
			return entry.rule, true
		}
	}
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/rpc/") {
		return rateRuleDefault, true
	}
	return RateLimitRule{}, false
}

const maxRateLimitBuckets = 50000

type rateBucket struct {
	tokens float64
	last   time.Time
	expiry time.Time
}

type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*rateBucket
	now       func() time.Time
	lastSweep time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rateBucket),
		now:     time.Now,
	}
}

func (rl *RateLimiter) Allow(key string, rule RateLimitRule) (allowed bool, retryAfter time.Duration) {
	rate := rule.ratePerSecond()
	if rate <= 0 || rule.Burst <= 0 {
		return true, 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.now()
	rl.sweepLocked(now)

	bucketKey := rule.Name + "|" + key
	bucket, found := rl.buckets[bucketKey]
	if !found {
		bucket = &rateBucket{tokens: float64(rule.Burst), last: now}
		rl.buckets[bucketKey] = bucket
	} else {
		elapsed := now.Sub(bucket.last).Seconds()
		if elapsed > 0 {
			bucket.tokens = math.Min(float64(rule.Burst), bucket.tokens+elapsed*rate)
		}
		bucket.last = now
	}

	if bucket.tokens >= 1 {
		bucket.tokens--
		allowed = true
	} else {
		wait := (1 - bucket.tokens) / rate
		retryAfter = time.Duration(math.Ceil(wait)) * time.Second
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
	}

	refill := (float64(rule.Burst) - bucket.tokens) / rate
	bucket.expiry = now.Add(time.Duration(refill * float64(time.Second)))
	return allowed, retryAfter
}

func (rl *RateLimiter) sweepLocked(now time.Time) {
	if now.Sub(rl.lastSweep) < time.Minute && len(rl.buckets) < maxRateLimitBuckets {
		return
	}
	rl.lastSweep = now

	for key, bucket := range rl.buckets {
		if !bucket.expiry.After(now) {
			delete(rl.buckets, key)
		}
	}

	if len(rl.buckets) >= maxRateLimitBuckets {
		LogWarn(LogCategorySystem, "Rate limiter bucket cap reached; resetting counters", map[string]interface{}{
			"buckets": len(rl.buckets),
		})
		rl.buckets = make(map[string]*rateBucket)
	}
}

type RateLimitWrapper struct {
	next    http.Handler
	limiter *RateLimiter
	enabled bool
}

func NewRateLimitWrapper(next http.Handler) *RateLimitWrapper {
	return &RateLimitWrapper{
		next:    next,
		limiter: NewRateLimiter(),
		enabled: rateLimitingEnabled(),
	}
}

func rateLimitingEnabled() bool {
	if cfg.IsRelease {
		return true
	}
	return os.Getenv("RATE_LIMIT_DISABLED") == ""
}

func (rw *RateLimitWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !rw.enabled {
		rw.next.ServeHTTP(w, r)
		return
	}

	rule, limited := ruleForPath(r.URL.Path)
	if !limited {
		rw.next.ServeHTTP(w, r)
		return
	}

	allowed, retryAfter := rw.limiter.Allow(rateLimitClientKey(r), rule)
	if allowed {
		rw.next.ServeHTTP(w, r)
		return
	}

	LogWarnWithRequest(r, LogCategorySystem, "Rate limit exceeded", map[string]interface{}{
		"rule":       rule.Name,
		"path":       r.URL.Path,
		"retryAfter": retryAfter.Seconds(),
	})
	respondRateLimited(w, r, retryAfter)
}

const rateLimitMessage = "Too many requests. Please wait a moment and try again."

func respondRateLimited(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	if r.URL.Path == SnapshotPath {
		w.Header().Set("Cache-Control", "no-store")
		http.NotFound(w, r)
		return
	}

	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	w.Header().Set("Cache-Control", "no-store")

	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   rateLimitMessage,
			"code":    ErrCodeRateLimited,
		})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(rateLimitMessage))
}

func rateLimitClientKey(r *http.Request) string {
	peer := peerIP(r)
	if peer == nil {
		return "unknown"
	}

	if isTrustedProxy(peer) {
		if forwarded := rightmostForwardedIP(r.Header.Get("X-Forwarded-For")); forwarded != nil {
			return clientKeyForIP(forwarded)
		}
		if realIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); realIP != nil {
			return clientKeyForIP(realIP)
		}
	}

	return clientKeyForIP(peer)
}

func peerIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func isTrustedProxy(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func rightmostForwardedIP(header string) net.IP {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		if ip := net.ParseIP(strings.TrimSpace(parts[i])); ip != nil {
			return ip
		}
	}
	return nil
}

func clientKeyForIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	if v6 := ip.To16(); v6 != nil {
		return v6.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip.String()
}
