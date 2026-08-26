package backend

import (
	"encoding/json"
	"family/cfg"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time          { return c.now }
func (c *fixedClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestLimiter() (*RateLimiter, *fixedClock) {
	clock := &fixedClock{now: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)}
	limiter := NewRateLimiter()
	limiter.now = clock.Now
	return limiter, clock
}

func TestRuleForPath(t *testing.T) {
	tests := []struct {
		path      string
		wantRule  string
		wantFound bool
	}{
		{path: "/api/login", wantRule: rateRuleLogin.Name, wantFound: true},
		{path: "/api/login/google/token", wantRule: rateRuleLogin.Name, wantFound: true},
		{path: "/api/refresh", wantRule: rateRuleRefresh.Name, wantFound: true},
		{path: "/rpc/CreateAccount", wantRule: rateRuleSignup.Name, wantFound: true},
		{path: "/rpc/RequestPasswordReset", wantRule: rateRulePasswordReset.Name, wantFound: true},
		{path: "/rpc/ResetPassword", wantRule: rateRulePasswordReset.Name, wantFound: true},
		{path: "/rpc/JoinFamily", wantRule: rateRuleInviteCode.Name, wantFound: true},
		{path: "/rpc/AcceptFamilyLink", wantRule: rateRuleInviteCode.Name, wantFound: true},
		{path: "/rpc/ProcessAIImport", wantRule: rateRuleAI.Name, wantFound: true},
		{path: "/rpc/ImportData", wantRule: rateRuleImport.Name, wantFound: true},
		{path: "/api/import-bundle", wantRule: rateRuleImport.Name, wantFound: true},
		{path: "/api/upload-photo", wantRule: rateRuleUpload.Name, wantFound: true},
		{path: "/ws/chat", wantRule: rateRuleWebSocket.Name, wantFound: true},
		{path: "/api/photo/42/full", wantRule: rateRulePhotoRead.Name, wantFound: true},
		{path: "/rpc/ListPeople", wantRule: rateRuleDefault.Name, wantFound: true},
		{path: "/api/anything-added-later", wantRule: rateRuleDefault.Name, wantFound: true},
		{path: "/static/app.js", wantFound: false},
		{path: "/dashboard", wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rule, found := ruleForPath(tt.path)
			if found != tt.wantFound {
				t.Fatalf("ruleForPath(%q) found = %v, want %v", tt.path, found, tt.wantFound)
			}
			if found && rule.Name != tt.wantRule {
				t.Errorf("ruleForPath(%q) rule = %q, want %q", tt.path, rule.Name, tt.wantRule)
			}
		})
	}
}

func TestEveryProtectedEndpointHasARule(t *testing.T) {
	protected := []string{
		"/api/login",
		"/rpc/CreateAccount",
		"/rpc/RequestPasswordReset",
		"/rpc/ResetPassword",
		"/rpc/JoinFamily",
		"/api/refresh",
		"/api/login/google/token",
		"/rpc/ProcessAIImport",
		"/rpc/ImportData",
		"/api/import-bundle",
		"/api/upload-photo",
		"/ws/chat",
	}

	for _, path := range protected {
		if _, found := ruleForPath(path); !found {
			t.Errorf("ruleForPath(%q) found no rule", path)
		}
	}
}

func TestRateLimiterAllowsBurstThenBlocks(t *testing.T) {
	limiter, _ := newTestLimiter()
	rule := RateLimitRule{Name: "test", Burst: 3, Window: time.Minute}

	for i := 0; i < rule.Burst; i++ {
		if allowed, _ := limiter.Allow("client", rule); !allowed {
			t.Fatalf("Allow() request %d denied inside the burst", i+1)
		}
	}

	allowed, retryAfter := limiter.Allow("client", rule)
	if allowed {
		t.Fatal("Allow() permitted a request past the burst")
	}
	if retryAfter <= 0 {
		t.Errorf("Allow() retryAfter = %v, want a positive duration", retryAfter)
	}
}

func TestRateLimiterRefillsOverTime(t *testing.T) {
	limiter, clock := newTestLimiter()
	rule := RateLimitRule{Name: "test", Burst: 2, Window: time.Minute}

	limiter.Allow("client", rule)
	limiter.Allow("client", rule)
	if allowed, _ := limiter.Allow("client", rule); allowed {
		t.Fatal("Allow() permitted a request past the burst")
	}

	clock.Advance(30 * time.Second)
	if allowed, _ := limiter.Allow("client", rule); !allowed {
		t.Fatal("Allow() denied a request after the bucket refilled")
	}
	if allowed, _ := limiter.Allow("client", rule); allowed {
		t.Fatal("Allow() refilled more than elapsed time earned")
	}
}

func TestRateLimiterRetryAfterIsLongEnough(t *testing.T) {
	limiter, clock := newTestLimiter()
	rule := RateLimitRule{Name: "test", Burst: 1, Window: time.Minute}

	limiter.Allow("client", rule)
	allowed, retryAfter := limiter.Allow("client", rule)
	if allowed {
		t.Fatal("Allow() permitted a second request against a burst of 1")
	}

	clock.Advance(retryAfter)
	if allowed, _ := limiter.Allow("client", rule); !allowed {
		t.Fatalf("Allow() denied a caller that waited the advertised %v", retryAfter)
	}
}

func TestRateLimiterSeparatesKeysAndRules(t *testing.T) {
	limiter, _ := newTestLimiter()
	rule := RateLimitRule{Name: "test", Burst: 1, Window: time.Minute}
	other := RateLimitRule{Name: "other", Burst: 1, Window: time.Minute}

	limiter.Allow("client-a", rule)
	if allowed, _ := limiter.Allow("client-b", rule); !allowed {
		t.Error("Allow() charged one client's usage to another")
	}
	if allowed, _ := limiter.Allow("client-a", other); !allowed {
		t.Error("Allow() charged one rule's usage to another")
	}
}

func TestRateLimiterSweepsRefilledBuckets(t *testing.T) {
	limiter, clock := newTestLimiter()
	rule := RateLimitRule{Name: "test", Burst: 2, Window: time.Minute}

	limiter.Allow("client", rule)
	if got := len(limiter.buckets); got != 1 {
		t.Fatalf("len(buckets) = %d, want 1", got)
	}

	clock.Advance(2 * time.Minute)
	limiter.Allow("other-client", rule)
	if _, found := limiter.buckets["test|client"]; found {
		t.Error("sweep kept a bucket that had refilled to full")
	}
}

func wrapperUnderTest(t *testing.T) (*RateLimitWrapper, *fixedClock, *int) {
	t.Helper()
	reached := 0
	wrapper := NewRateLimitWrapper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		w.WriteHeader(http.StatusOK)
	}))
	wrapper.enabled = true
	clock := &fixedClock{now: time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)}
	wrapper.limiter.now = clock.Now
	return wrapper, clock, &reached
}

func postTo(path, remoteAddr string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.RemoteAddr = remoteAddr
	return req
}

func TestRateLimitWrapperBlocksAfterBurst(t *testing.T) {
	wrapper, _, reached := wrapperUnderTest(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < rateRuleLogin.Burst+1; i++ {
		last = httptest.NewRecorder()
		wrapper.ServeHTTP(last, postTo("/api/login", "203.0.113.10:5000"))
	}

	if *reached != rateRuleLogin.Burst {
		t.Errorf("handler saw %d requests, want %d", *reached, rateRuleLogin.Burst)
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", last.Code, http.StatusTooManyRequests)
	}

	retryAfter := last.Header().Get("Retry-After")
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil || seconds < 1 {
		t.Errorf("Retry-After = %q, want a positive number of seconds", retryAfter)
	}
}

func TestRateLimitWrapperAnswersAPIRequestsWithJSON(t *testing.T) {
	wrapper, _, _ := wrapperUnderTest(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < rateRuleLogin.Burst+1; i++ {
		last = httptest.NewRecorder()
		wrapper.ServeHTTP(last, postTo("/api/login", "203.0.113.11:5000"))
	}

	var body struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.NewDecoder(last.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Success {
		t.Error("rate-limited login response reported success")
	}
	if body.Error == "" {
		t.Error("rate-limited login response carried no error message")
	}
}

func TestRateLimitWrapperAnswersRPCRequestsWithText(t *testing.T) {
	wrapper, _, _ := wrapperUnderTest(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < rateRuleSignup.Burst+1; i++ {
		last = httptest.NewRecorder()
		wrapper.ServeHTTP(last, postTo("/rpc/CreateAccount", "203.0.113.12:5000"))
	}

	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", last.Code, http.StatusTooManyRequests)
	}
	if got := last.Body.String(); got != rateLimitMessage {
		t.Errorf("body = %q, want the plain-text rate limit message", got)
	}
}

func TestRateLimitMessageRevealsNothing(t *testing.T) {
	for _, leak := range []string{"login", "signup", "bucket", "token", "burst"} {
		if strings.Contains(strings.ToLower(rateLimitMessage), leak) {
			t.Errorf("rate limit message mentions %q", leak)
		}
	}
}

func TestRateLimitWrapperKeepsSnapshotEndpointHidden(t *testing.T) {
	wrapper, _, _ := wrapperUnderTest(t)

	var last *httptest.ResponseRecorder
	for i := 0; i < rateRuleSnapshot.Burst+1; i++ {
		last = httptest.NewRecorder()
		wrapper.ServeHTTP(last, postTo(SnapshotPath, "203.0.113.30:5000"))
	}

	if last.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", last.Code, http.StatusNotFound)
	}
	if got := last.Header().Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want no header", got)
	}
}

func TestRateLimitWrapperLeavesUnlimitedPathsAlone(t *testing.T) {
	wrapper, _, reached := wrapperUnderTest(t)

	for i := 0; i < rateRuleDefault.Burst*2; i++ {
		recorder := httptest.NewRecorder()
		wrapper.ServeHTTP(recorder, postTo("/static/app.js", "203.0.113.13:5000"))
		if recorder.Code != http.StatusOK {
			t.Fatalf("static asset request %d got status %d", i+1, recorder.Code)
		}
	}
	if *reached != rateRuleDefault.Burst*2 {
		t.Errorf("handler saw %d static requests, want %d", *reached, rateRuleDefault.Burst*2)
	}
}

func TestRateLimitWrapperSeparatesClients(t *testing.T) {
	wrapper, _, _ := wrapperUnderTest(t)

	for i := 0; i < rateRuleLogin.Burst; i++ {
		wrapper.ServeHTTP(httptest.NewRecorder(), postTo("/api/login", "203.0.113.20:5000"))
	}

	recorder := httptest.NewRecorder()
	wrapper.ServeHTTP(recorder, postTo("/api/login", "203.0.113.21:5000"))
	if recorder.Code != http.StatusOK {
		t.Fatalf("second client got status %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRateLimitClientKey(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "direct connection uses the peer address",
			remoteAddr: "203.0.113.5:4000",
			want:       "203.0.113.5",
		},
		{
			name:       "spoofed forwarded header is ignored from an untrusted peer",
			remoteAddr: "203.0.113.5:4000",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.9"},
			want:       "203.0.113.5",
		},
		{
			name:       "forwarded header is trusted behind the local proxy",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.9"},
			want:       "198.51.100.9",
		},
		{
			name:       "rightmost forwarded entry wins over a client-supplied one",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "10.9.9.9, 198.51.100.9"},
			want:       "198.51.100.9",
		},
		{
			name:       "real-ip is used when no forwarded header is present",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Real-IP": "198.51.100.20"},
			want:       "198.51.100.20",
		},
		{
			name:       "garbage forwarded header falls back to the peer",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "not-an-ip"},
			want:       "127.0.0.1",
		},
		{
			name:       "ipv6 clients are grouped by /64",
			remoteAddr: "127.0.0.1:4000",
			headers:    map[string]string{"X-Forwarded-For": "2001:db8:1:2:3:4:5:6"},
			want:       "2001:db8:1:2::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/login", nil)
			req.RemoteAddr = tt.remoteAddr
			for name, value := range tt.headers {
				req.Header.Set(name, value)
			}

			if got := rateLimitClientKey(req); got != tt.want {
				t.Errorf("rateLimitClientKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRateLimitGroupsIPv6Subscribers(t *testing.T) {
	wrapper, _, _ := wrapperUnderTest(t)

	for i := 0; i < rateRuleLogin.Burst; i++ {
		req := postTo("/api/login", "127.0.0.1:4000")
		req.Header.Set("X-Forwarded-For", "2001:db8:1:2::"+strconv.Itoa(i+1))
		wrapper.ServeHTTP(httptest.NewRecorder(), req)
	}

	req := postTo("/api/login", "127.0.0.1:4000")
	req.Header.Set("X-Forwarded-For", "2001:db8:1:2::ffff")
	recorder := httptest.NewRecorder()
	wrapper.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want %d for another address in the same /64", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimitingCannotBeDisabledInReleaseBuilds(t *testing.T) {
	t.Setenv("RATE_LIMIT_DISABLED", "1")

	got := rateLimitingEnabled()
	if cfg.IsRelease && !got {
		t.Fatal("rateLimitingEnabled() honored the opt-out in a release build")
	}
	if !cfg.IsRelease && got {
		t.Fatal("rateLimitingEnabled() ignored the opt-out in a local build")
	}
}
