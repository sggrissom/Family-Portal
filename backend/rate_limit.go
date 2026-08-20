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

// Signup is open to the internet and every expensive path in the app — password
// hashing, photo processing, an outbound AI call, a full-archive import — sits
// behind an endpoint anyone can POST to. The limits below are per client, per
// endpoint group, and are sized so that a person using the site never reaches
// them: they exist to make automated abuse slow, not to police normal use.
//
// The buckets are in-process. One server, one process, no shared cache to keep
// running; if the process restarts the limits reset, which is an acceptable
// trade for not adding infrastructure to a family photo site.

// RateLimitRule is a token bucket: Burst requests may arrive at once, and a
// fully drained bucket refills over Window.
type RateLimitRule struct {
	// Name identifies the bucket. Endpoints sharing a name share a budget,
	// which is what stops an attacker from getting a fresh allowance per path.
	Name string
	// Burst is the bucket capacity — the most requests allowed back to back.
	Burst int
	// Window is how long a fully drained bucket takes to refill completely.
	Window time.Duration
}

// ratePerSecond is the bucket's refill rate.
func (r RateLimitRule) ratePerSecond() float64 {
	if r.Window <= 0 {
		return 0
	}
	return float64(r.Burst) / r.Window.Seconds()
}

// The rules. Credential-guessing paths are deliberately the tightest; the paths
// a real session hits repeatedly (photo fetches, ordinary RPCs) are loose enough
// that a large gallery or a busy dashboard never notices them.
var (
	// Password guessing. Someone mistyping their password a few times stays
	// well inside this; a dictionary attack does not.
	rateRuleLogin = RateLimitRule{Name: "login", Burst: 10, Window: 5 * time.Minute}
	// Account creation is rare per household, even on a shared address.
	rateRuleSignup = RateLimitRule{Name: "signup", Burst: 5, Window: time.Hour}
	// Reset requests mail a token; reset attempts guess one.
	rateRulePasswordReset = RateLimitRule{Name: "password-reset", Burst: 5, Window: 15 * time.Minute}
	// Invite and link codes are short, so guessing them must be slow.
	rateRuleInviteCode = RateLimitRule{Name: "invite-code", Burst: 10, Window: 15 * time.Minute}
	// Refresh is automatic and fires per tab, so it gets more room than login
	// while still bounding a token-grinding loop.
	rateRuleRefresh = RateLimitRule{Name: "refresh", Burst: 30, Window: 5 * time.Minute}
	// Each AI call costs money at an external provider.
	rateRuleAI = RateLimitRule{Name: "ai", Burst: 10, Window: time.Hour}
	// Imports are the heaviest thing a user can ask for: a whole archive
	// unpacked, decoded, and written.
	rateRuleImport = RateLimitRule{Name: "import", Burst: 5, Window: time.Hour}
	// Bulk-uploading a holiday's worth of photos is normal, so this is sized
	// for a big batch rather than a single picture.
	rateRuleUpload = RateLimitRule{Name: "upload", Burst: 120, Window: 10 * time.Minute}
	// Chat sockets reconnect on wake, network changes, and redeploys.
	rateRuleWebSocket = RateLimitRule{Name: "websocket", Burst: 30, Window: 5 * time.Minute}
	// Photo GETs are the one thing a single page view fires dozens of.
	rateRulePhotoRead = RateLimitRule{Name: "photo-read", Burst: 600, Window: 5 * time.Minute}
	// The snapshot endpoint guards a bearer token and streams the whole
	// database. Nightly backups need a handful of attempts; a token-guessing
	// loop needs thousands.
	rateRuleSnapshot = RateLimitRule{Name: "snapshot", Burst: 10, Window: time.Hour}
	// Everything else under /api/ and /rpc/. A catch-all so an endpoint added
	// later is bounded by default instead of unbounded by omission.
	rateRuleDefault = RateLimitRule{Name: "default", Burst: 300, Window: time.Minute}
)

// exactPathRules matches whole paths.
var exactPathRules = map[string]RateLimitRule{
	"/api/login":              rateRuleLogin,
	"/api/login/google":       rateRuleLogin,
	"/api/login/google/token": rateRuleLogin,
	"/api/google/callback":    rateRuleLogin,
	"/api/refresh":            rateRuleRefresh,
	// Guessing the current password from inside a session is still password
	// guessing, so both of these are bounded the same way the login form is.
	"/api/change-password": rateRuleLogin,
	"/api/delete-account":  rateRuleLogin,
	"/api/upload-photo":    rateRuleUpload,
	"/api/import-bundle":   rateRuleImport,
	"/ws/chat":             rateRuleWebSocket,
	SnapshotPath:           rateRuleSnapshot,

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

// prefixPathRules matches path prefixes, longest listed first.
var prefixPathRules = []struct {
	prefix string
	rule   RateLimitRule
}{
	{prefix: "/api/photo/", rule: rateRulePhotoRead},
}

// ruleForPath returns the rule guarding a path, and whether one applies. Static
// assets and the frontend itself are unlimited: they are cheap, cacheable, and
// the reverse proxy is the right place to bound them.
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

// maxRateLimitBuckets bounds memory. Each bucket is a key and two numbers, so
// this is a few megabytes at worst — reached only under a distributed flood,
// which is the case the reset below exists for.
const maxRateLimitBuckets = 50000

type rateBucket struct {
	tokens float64
	// last is when tokens was computed.
	last time.Time
	// expiry is when the bucket will be full again. A full bucket is
	// indistinguishable from an absent one, so it can be swept.
	expiry time.Time
}

// RateLimiter holds the token buckets. The zero value is not usable; call
// NewRateLimiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	// now is injected so tests can advance time without sleeping.
	now       func() time.Time
	lastSweep time.Time
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		buckets: make(map[string]*rateBucket),
		now:     time.Now,
	}
}

// Allow consumes one token for key under rule. When it returns false, retryAfter
// is how long the caller must wait for a token to exist.
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
		// Round up so a caller that obeys Retry-After is not refused again.
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

// sweepLocked drops buckets that have refilled to full, since re-creating one is
// identical to keeping it. It runs at most once a minute so a busy server does
// not walk the map on every request.
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

	// Still over the cap means a flood from many distinct addresses. Dropping
	// everything costs one window of enforcement; growing without bound costs
	// the process.
	if len(rl.buckets) >= maxRateLimitBuckets {
		LogWarn(LogCategorySystem, "Rate limiter bucket cap reached; resetting counters", map[string]interface{}{
			"buckets": len(rl.buckets),
		})
		rl.buckets = make(map[string]*rateBucket)
	}
}

// RateLimitWrapper applies the rules to every request before it reaches a
// handler. It sits outside the size-limit and security wrappers so a flood is
// refused before anything reads a body or touches the database.
type RateLimitWrapper struct {
	next    http.Handler
	limiter *RateLimiter
	// enabled is false only in local builds that opt out; release builds
	// ignore the opt-out entirely.
	enabled bool
}

func NewRateLimitWrapper(next http.Handler) *RateLimitWrapper {
	return &RateLimitWrapper{
		next:    next,
		limiter: NewRateLimiter(),
		enabled: rateLimitingEnabled(),
	}
}

// rateLimitingEnabled reports whether limits are applied. The opt-out exists for
// local load testing and end-to-end runs; a release build never honors it,
// because a production server that silently stopped limiting would look exactly
// like one that was working.
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

// rateLimitMessage is deliberately vague about which limit was hit and how much
// budget remains: the useful detail goes to the logs, not to whoever is probing.
const rateLimitMessage = "Too many requests. Please wait a moment and try again."

// respondRateLimited answers in the shape the caller expects. The /api handlers
// return {success, error} objects that the frontend reads directly, while /rpc
// errors are bare text that vlens surfaces as the error string.
func respondRateLimited(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	// The snapshot endpoint answers unauthorized callers with 404 so it is not
	// discoverable. A 429 there would announce that something exists at that
	// path, so the limit keeps the same disguise.
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
			// The code lets a client tell "slow down and retry" apart from
			// "you did something wrong"; Retry-After above says when.
			"code": ErrCodeRateLimited,
		})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_, _ = w.Write([]byte(rateLimitMessage))
}

// rateLimitClientKey identifies the caller for limiting purposes.
//
// This is not getClientIP: that one trusts X-Forwarded-For unconditionally,
// which is fine for a log line and useless for a limit, since anyone can send a
// fresh value per request and mint themselves an unlimited number of budgets.
// Here the header is read only when the connection came from a proxy we run,
// and only its rightmost entry — the one that proxy appended itself.
func rateLimitClientKey(r *http.Request) string {
	peer := peerIP(r)
	if peer == nil {
		// No usable peer address: bucket these together rather than letting
		// them through unlimited.
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

// isTrustedProxy reports whether the direct peer is infrastructure rather than a
// user. Caddy proxies the public domain to localhost, so in production every
// request legitimately arrives from a loopback address.
func isTrustedProxy(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// rightmostForwardedIP returns the last address in an X-Forwarded-For header.
// A proxy appends the address it received the connection from, so anything a
// client puts in the header ends up to the left of the value we want.
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

// clientKeyForIP collapses an address to the unit being limited. IPv4 is limited
// per address; IPv6 is limited per /64, because a single subscriber is routinely
// handed that whole range and could otherwise use a new address per request.
func clientKeyForIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	if v6 := ip.To16(); v6 != nil {
		return v6.Mask(net.CIDRMask(64, 128)).String() + "/64"
	}
	return ip.String()
}
