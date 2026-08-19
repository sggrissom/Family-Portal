// Command smokecheck exercises a deployed Family Portal over HTTP the way a
// browser does, then exits nonzero if anything a user would notice is broken.
//
// It is the post-deploy gate described in docs/deployment.md. The deploy
// script's own health check only asks whether the process answers /healthz,
// which a build that serves a stale frontend, cannot reach its database, or
// refuses every login still passes. This asks the questions that come after
// that: does the site render, does a real account get in, does the database
// answer, does a photo come back off disk, does the chat socket accept a
// connection.
//
// Every check is read-only. The single piece of state it creates is a login
// session, which the final check disposes of through /api/logout.
//
//	smokecheck -url https://familyrecord.app -email smoke@example.com -password ...
//
// The credentials belong to a real account in a real family — one with at
// least one processed photo, because "a photo loads" cannot be answered
// without one. A dedicated account is worth creating so a failure here is
// never confused with a person's own session.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// maxResponseBytes bounds what a single check will read. Photos are the
// largest thing fetched here and a full-size original stays well under this.
const maxResponseBytes = 16 << 20

// browserAccept mirrors what a modern browser offers for an <img> request, so
// the server picks the same encoded variant it would serve a real visitor.
const browserAccept = "image/avif,image/webp,image/apng,image/*,*/*;q=0.8"

// scriptSrc finds the module the built index.html loads. The filename carries
// a content hash that changes with every frontend build, which is what makes
// fetching it worthwhile: an index.html and a dist directory from different
// deploys will disagree here and nowhere else.
var scriptSrc = regexp.MustCompile(`<script[^>]+src="([^"]+)"`)

type smoker struct {
	base     *url.URL
	http     *http.Client
	email    string
	password string

	// loggedIn gates the checks that need a session, so a login failure
	// reports the later checks as skipped rather than as four more failures
	// with the same cause.
	loggedIn bool
}

func main() {
	base := flag.String("url", os.Getenv("SMOKE_URL"), "base URL of the deployment to check")
	email := flag.String("email", os.Getenv("SMOKE_EMAIL"), "email of the account to sign in as")
	password := flag.String("password", os.Getenv("SMOKE_PASSWORD"), "password for that account")
	timeout := flag.Duration("timeout", 60*time.Second, "deadline for the whole run")
	flag.Parse()

	if *base == "" || *email == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "smokecheck: -url, -email, and -password are all required")
		fmt.Fprintln(os.Stderr, "(SMOKE_URL, SMOKE_EMAIL, and SMOKE_PASSWORD are read as defaults)")
		flag.Usage()
		os.Exit(2)
	}

	s, err := newSmoker(*base, *email, *password, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smokecheck: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	checks := []struct {
		name string
		run  func(context.Context) (string, error)
	}{
		{"readyz", s.checkReadyz},
		{"landing page", s.checkLanding},
		{"login", s.checkLogin},
		{"photo", s.checkPhoto},
		{"websocket", s.checkWebSocket},
		{"logout", s.checkLogout},
	}

	fmt.Printf("smoke checking %s\n", s.base)
	failures := 0
	for _, check := range checks {
		started := time.Now()
		detail, err := check.run(ctx)
		elapsed := time.Since(started).Round(time.Millisecond)

		switch {
		case errors.Is(err, errSkipped):
			fmt.Printf("skip %-13s %v\n", check.name, err)
		case err != nil:
			fmt.Printf("FAIL %-13s %v (%s)\n", check.name, err, elapsed)
			failures++
		default:
			fmt.Printf("ok   %-13s %s (%s)\n", check.name, detail, elapsed)
		}
	}

	if failures > 0 {
		fmt.Printf("\n%d of %d checks failed\n", failures, len(checks))
		os.Exit(1)
	}
	fmt.Printf("\nall %d checks passed\n", len(checks))
}

func newSmoker(base, email, password string, timeout time.Duration) (*smoker, error) {
	parsed, err := url.Parse(strings.TrimSuffix(base, "/"))
	if err != nil {
		return nil, fmt.Errorf("bad -url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("bad -url %q: want a scheme and host, e.g. https://familyrecord.app", base)
	}

	// The jar is how the session survives between checks: the login handler
	// sets authToken as a cookie and every later request has to present it,
	// exactly as the browser does.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	return &smoker{
		base:     parsed,
		http:     &http.Client{Jar: jar, Timeout: timeout},
		email:    email,
		password: password,
	}, nil
}

// errSkipped marks a check that could not be attempted, as opposed to one that
// was attempted and failed. Wrap it to say why.
var errSkipped = errors.New("skipped")

func (s *smoker) checkReadyz(ctx context.Context) (string, error) {
	resp, body, err := s.get(ctx, "/readyz", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}
	if got := strings.TrimSpace(string(body)); got != "ok" {
		return "", fmt.Errorf("unexpected body %q", got)
	}
	return "database readable, storage writable", nil
}

func (s *smoker) checkLanding(ctx context.Context) (string, error) {
	resp, body, err := s.get(ctx, "/", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}
	if ctype := resp.Header.Get("Content-Type"); !strings.Contains(ctype, "text/html") {
		return "", fmt.Errorf("content-type %q, want text/html", ctype)
	}

	match := scriptSrc.FindSubmatch(body)
	if match == nil {
		return "", errors.New("no <script src> in the page; the frontend build did not land")
	}
	src := string(match[1])

	// A deploy that shipped index.html without its bundle, or with the
	// previous build's bundle, is a blank page for every visitor. This is the
	// check that catches it.
	scriptResp, scriptBody, err := s.get(ctx, src, nil)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", src, err)
	}
	if scriptResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: status %d", src, scriptResp.StatusCode)
	}
	if len(scriptBody) == 0 {
		return "", fmt.Errorf("%s is empty", src)
	}

	return fmt.Sprintf("index.html + %s (%s)", src, byteCount(len(scriptBody))), nil
}

type loginResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (s *smoker) checkLogin(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{"email": s.email, "password": s.password})
	if err != nil {
		return "", err
	}

	resp, body, err := s.post(ctx, "/api/login", payload)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}

	var parsed loginResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("response was not JSON: %s", excerpt(body))
	}
	if !parsed.Success {
		return "", fmt.Errorf("login rejected: %s", parsed.Error)
	}
	if !s.hasAuthCookie() {
		return "", errors.New("login succeeded but set no authToken cookie")
	}

	s.loggedIn = true
	return fmt.Sprintf("signed in as %s", s.email), nil
}

type photoListResponse struct {
	Photos []struct {
		Image struct {
			Id     int `json:"id"`
			Status int `json:"status"`
		} `json:"image"`
	} `json:"photos"`
}

func (s *smoker) checkPhoto(ctx context.Context) (string, error) {
	if !s.loggedIn {
		return "", fmt.Errorf("%w: no session", errSkipped)
	}

	// Listing goes through the RPC layer and reads the database under the
	// caller's family scope, so a failure here separates "the database is
	// unreadable" from "the file is missing" before a byte of image is asked
	// for.
	resp, body, err := s.post(ctx, "/rpc/ListFamilyPhotos", []byte(`{"personId":0}`))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ListFamilyPhotos: status %d: %s", resp.StatusCode, excerpt(body))
	}

	var listed photoListResponse
	if err := json.Unmarshal(body, &listed); err != nil {
		return "", fmt.Errorf("ListFamilyPhotos returned %s", excerpt(body))
	}

	photoId := 0
	for _, photo := range listed.Photos {
		// Status 0 is a finished photo; 1 is still in the worker queue and
		// serves a placeholder, which would make this check pass on a photo
		// that never processed.
		if photo.Image.Status == 0 {
			photoId = photo.Image.Id
			break
		}
	}
	if photoId == 0 {
		return "", fmt.Errorf("no processed photo in this account's family (%d listed); "+
			"upload one, or the check cannot tell whether photos serve", len(listed.Photos))
	}

	header := http.Header{"Accept": {browserAccept}}
	total := 0
	for _, path := range []string{
		fmt.Sprintf("/api/photo/%d/thumb", photoId),
		fmt.Sprintf("/api/photo/%d", photoId),
	} {
		imgResp, imgBody, err := s.get(ctx, path, header)
		if err != nil {
			return "", err
		}
		if imgResp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("%s: status %d: %s", path, imgResp.StatusCode, excerpt(imgBody))
		}
		if ctype := imgResp.Header.Get("Content-Type"); !strings.HasPrefix(ctype, "image/") {
			return "", fmt.Errorf("%s: content-type %q, want an image", path, ctype)
		}
		if len(imgBody) == 0 {
			return "", fmt.Errorf("%s: empty body", path)
		}
		total += len(imgBody)
	}

	return fmt.Sprintf("photo %d thumb + full (%s)", photoId, byteCount(total)), nil
}

func (s *smoker) checkWebSocket(ctx context.Context) (string, error) {
	if !s.loggedIn {
		return "", fmt.Errorf("%w: no session", errSkipped)
	}

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Origin is set because the server checks it and a browser always sends
	// it; dialing without one would skip the check that a real client faces.
	conn, resp, err := websocket.Dial(dialCtx, s.resolve("/ws/chat"), &websocket.DialOptions{
		HTTPClient: s.http,
		HTTPHeader: http.Header{"Origin": {s.base.String()}},
	})
	if err != nil {
		if resp != nil {
			return "", fmt.Errorf("handshake failed with status %d: %w", resp.StatusCode, err)
		}
		return "", err
	}
	defer conn.Close(websocket.StatusNormalClosure, "smoke check complete")

	// The handshake alone only proves the upgrade happened. A heartbeat that
	// comes back proves the connection was registered with the hub and that
	// both pumps are running.
	if err := wsjson.Write(dialCtx, conn, map[string]any{"type": "heartbeat"}); err != nil {
		return "", fmt.Errorf("sending heartbeat: %w", err)
	}

	// Presence broadcasts can arrive first; read past them for the reply.
	for {
		var msg struct {
			Type string `json:"type"`
		}
		if err := wsjson.Read(dialCtx, conn, &msg); err != nil {
			return "", fmt.Errorf("waiting for heartbeat reply: %w", err)
		}
		if msg.Type == "heartbeat" {
			return "connected, heartbeat answered", nil
		}
	}
}

func (s *smoker) checkLogout(ctx context.Context) (string, error) {
	if !s.loggedIn {
		return "", fmt.Errorf("%w: no session", errSkipped)
	}

	resp, body, err := s.post(ctx, "/api/logout", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}
	if s.hasAuthCookie() {
		return "", errors.New("logout left the authToken cookie in place")
	}
	return "session revoked", nil
}

func (s *smoker) resolve(path string) string {
	ref, err := url.Parse(path)
	if err != nil {
		return s.base.String() + path
	}
	return s.base.ResolveReference(ref).String()
}

func (s *smoker) hasAuthCookie() bool {
	for _, cookie := range s.http.Jar.Cookies(s.base) {
		if cookie.Name == "authToken" && cookie.Value != "" {
			return true
		}
	}
	return false
}

func (s *smoker) get(ctx context.Context, path string, header http.Header) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.resolve(path), nil)
	if err != nil {
		return nil, nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	return s.do(req)
}

func (s *smoker) post(ctx context.Context, path string, body []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.resolve(path), bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req)
}

func (s *smoker) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return resp, nil, fmt.Errorf("reading %s: %w", req.URL.Path, err)
	}
	return resp, body, nil
}

// excerpt renders an error body short enough to sit on one line. Bodies here
// come from a server that is already misbehaving, so they may be anything.
func excerpt(body []byte) string {
	text := strings.TrimSpace(string(body))
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		text = text[:idx]
	}
	if len(text) > 120 {
		text = text[:120] + "..."
	}
	if text == "" {
		return "(empty body)"
	}
	return text
}

func byteCount(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
