//go:build release

// Command e2e drives the five flows a release cannot ship broken — signup and
// login, adding a person, recording a growth measurement, uploading a photo,
// and chat — against the compiled release binary, over TLS, on a scratch
// deployment that it creates and removes.
//
// The unit tests call handlers in-process against a local build. That leaves a
// gap this fills: what actually ships is a release-tagged binary with the
// frontend embedded in it, compile-time storage paths, config checks that
// refuse to serve, rate limiting that cannot be switched off, a face analysis
// worker that really does look for its daemon, and Secure cookies that no
// plaintext client can hold. None of that is exercised by `go test ./backend/`,
// and every one of them has broken a deploy somewhere.
//
// Two pieces of the production shape are reproduced here. Requests arrive over
// https through a reverse proxy, because Caddy terminates TLS in front of the
// app (docs/deployment.md) and because the auth cookie is Secure, so a client
// talking plain http would silently never send it. And the binary runs as its
// own process, so the run also answers whether it starts, drains on SIGTERM,
// and exits zero.
//
// A release build resolves its storage paths at compile time (cfg/release.go),
// so the scratch deployment has to live where production would:
//
//	sudo mkdir -p /srv/apps/family/shared/data /srv/apps/family/shared/static \
//	  /srv/apps/family/shared/logs
//	sudo chown -R "$(id -un)" /srv/apps/family/shared
//
// On a machine where that tree would be unwelcome, bwrap can supply a throwaway
// one without sudo; docs/deployment.md has the invocation.
//
// It refuses to start if the tree already holds a database, a shared .env, or
// anything in the static directory, which is what keeps it from ever running
// against a real deployment. A refusal deletes nothing. Everything a run that
// did start creates is removed again, unless -keep says otherwise.
//
//	make e2e
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"family/cfg"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	// appPort is the port the release binary listens on. It is a constant in
	// the binary too (release/release.go), which is why nothing here is
	// configurable: there is no flag that would move it.
	appPort = 8666

	// startupTimeout covers process start, the config check, opening the
	// database, and running every migration against an empty one.
	startupTimeout = 60 * time.Second

	// photoTimeout covers the background worker encoding six size variants of
	// the uploaded image. The test image is tiny; the budget is for a loaded
	// CI runner, not for the work.
	photoTimeout = 90 * time.Second

	// shutdownTimeout is the application's own drain budget plus room to
	// notice. A binary that needs longer than this is the defect.
	shutdownTimeout = 45 * time.Second

	maxResponseBytes = 16 << 20
	maxLogLines      = 200
)

// browserAccept mirrors what a browser offers for an <img> request, so the
// server picks the same encoded variant it would serve a real visitor.
const browserAccept = "image/avif,image/webp,image/apng,image/*,*/*;q=0.8"

// scriptSrc finds the module the built index.html loads. The filename carries a
// content hash, so fetching it proves the HTML and the embedded dist came from
// the same build rather than merely that both exist.
var scriptSrc = regexp.MustCompile(`<script[^>]+src="([^"]+)"`)

func main() {
	binary := flag.String("binary", filepath.Join("build", "family_site"), "release binary to run")
	keep := flag.Bool("keep", false, "leave the scratch deployment and the server's working directory behind")
	echo := flag.Bool("echo", false, "stream the server's own log to stderr as it runs")
	timeout := flag.Duration("timeout", 5*time.Minute, "deadline for the whole run")
	flag.Parse()

	h := &harness{
		binary:   *binary,
		keep:     *keep,
		log:      &serverLog{echo: *echo},
		email:    "e2e@family-portal.invalid",
		password: "e2e-portal-password",
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := h.start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		h.dumpServerLog()
		h.stop()
		h.cleanup()
		os.Exit(1)
	}

	runErr := h.run(ctx)

	// The shutdown is itself a check: a release that cannot drain on SIGTERM
	// takes a deploy's worth of in-flight requests with it every time.
	stopErr := h.stop()
	if runErr == nil && stopErr != nil {
		fmt.Printf("FAIL %-12s %v\n", "shutdown", stopErr)
		runErr = stopErr
	} else if runErr == nil {
		fmt.Printf("ok   %-12s SIGTERM drained, exit 0\n", "shutdown")
	}

	if runErr != nil {
		h.dumpServerLog()
	}
	h.cleanup()

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "\ne2e failed: %v\n", runErr)
		os.Exit(1)
	}
	fmt.Println("\ne2e: all flows passed")
}

type harness struct {
	binary string
	keep   bool
	log    *serverLog

	// dir is the server's working directory. A release build writes its
	// rotating log and its relative data/static directories there, so it gets
	// a scratch one rather than the repository.
	dir   string
	addr  string
	cmd   *exec.Cmd
	front *httptest.Server
	base  *url.URL
	http  *http.Client

	// exit carries the result of the one permitted cmd.Wait. It is read
	// without blocking during the run so a server that died is reported as
	// itself, and drained by stop when the run is over.
	exit    chan error
	exited  bool
	exitErr error

	// owned records that preflight proved the deployment tree unused. It is
	// what the cleanup checks before deleting anything there: without it, a
	// harness pointed at a real deployment would refuse to run and then
	// remove the database it had just refused to touch.
	owned bool

	email    string
	password string

	// Carried between steps, because each flow builds on the one before it.
	personId int
	photoId  int
}

func (h *harness) run(ctx context.Context) error {
	steps := []struct {
		name string
		fn   func(context.Context) (string, error)
	}{
		{"readyz", h.stepReadyz},
		{"landing", h.stepLanding},
		{"signup", h.stepSignup},
		{"login", h.stepLogin},
		{"person", h.stepAddPerson},
		{"growth", h.stepAddGrowth},
		{"photo", h.stepUploadPhoto},
		{"chat", h.stepChat},
		{"logout", h.stepLogout},
	}

	for _, step := range steps {
		started := time.Now()
		detail, err := step.fn(ctx)
		elapsed := time.Since(started).Round(time.Millisecond)
		if err != nil {
			fmt.Printf("FAIL %-12s %v (%s)\n", step.name, err, elapsed)
			// The steps are a chain — a person that was never created cannot
			// have growth recorded against it — so the run stops here rather
			// than reporting the same cause five more times.
			return fmt.Errorf("%s: %w", step.name, err)
		}
		fmt.Printf("ok   %-12s %s (%s)\n", step.name, detail, elapsed)

		if err := h.serverAlive(); err != nil {
			return err
		}
	}
	return nil
}

// ── the flows ──────────────────────────────────────────────────────────────

func (h *harness) stepReadyz(ctx context.Context) (string, error) {
	resp, body, err := h.get(ctx, "/readyz", nil)
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

func (h *harness) stepLanding(ctx context.Context) (string, error) {
	resp, body, err := h.get(ctx, "/", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}

	match := scriptSrc.FindSubmatch(body)
	if match == nil {
		return "", errors.New("no <script src> in the page; the frontend was not embedded")
	}
	src := string(match[1])

	scriptResp, scriptBody, err := h.get(ctx, src, nil)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", src, err)
	}
	if scriptResp.StatusCode != http.StatusOK || len(scriptBody) == 0 {
		return "", fmt.Errorf("%s: status %d, %d bytes", src, scriptResp.StatusCode, len(scriptBody))
	}
	return fmt.Sprintf("index.html + %s (%s)", src, byteCount(len(scriptBody))), nil
}

func (h *harness) stepSignup(ctx context.Context) (string, error) {
	var out struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
		Auth    struct {
			Id       int    `json:"id"`
			FamilyId int    `json:"familyId"`
			Name     string `json:"name"`
		} `json:"auth"`
	}
	err := h.rpc(ctx, "CreateAccount", map[string]any{
		"name":                   "E2E Runner",
		"email":                  h.email,
		"password":               h.password,
		"confirmPassword":        h.password,
		"familyCode":             "",
		"initialPersonName":      "E2E Parent",
		"initialPersonGender":    0,
		"initialPersonBirthdate": "1990-04-01",
	}, &out)
	if err != nil {
		return "", err
	}
	if !out.Success {
		return "", fmt.Errorf("account creation rejected: %s", out.Error)
	}
	if out.Auth.Id == 0 || out.Auth.FamilyId == 0 {
		return "", fmt.Errorf("account created without a user or family: %+v", out.Auth)
	}
	return fmt.Sprintf("user %d in family %d", out.Auth.Id, out.Auth.FamilyId), nil
}

func (h *harness) stepLogin(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{"email": h.email, "password": h.password})
	if err != nil {
		return "", err
	}
	resp, body, err := h.post(ctx, "/api/login", "application/json", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}

	var parsed struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("response was not JSON: %s", excerpt(body))
	}
	if !parsed.Success {
		return "", fmt.Errorf("login rejected: %s", parsed.Error)
	}
	// The cookie is Secure, so its arrival here also confirms the whole
	// request went over TLS the way a browser's would.
	if !h.hasCookie("authToken") {
		return "", errors.New("login succeeded but set no authToken cookie")
	}
	return "signed in, session cookie held", nil
}

func (h *harness) stepAddPerson(ctx context.Context) (string, error) {
	var added struct {
		Person struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		} `json:"person"`
	}
	err := h.rpc(ctx, "AddPerson", map[string]any{
		"name":        "E2E Child",
		"personType":  1,
		"gender":      1,
		"birthdate":   "2020-06-15",
		"isPregnancy": false,
		"familyId":    0,
	}, &added)
	if err != nil {
		return "", err
	}
	if added.Person.Id == 0 {
		return "", errors.New("AddPerson returned no person")
	}
	h.personId = added.Person.Id

	// Read it back through a separate call, so the step reports that the
	// person was stored rather than only that a handler answered.
	var fetched struct {
		Person struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
		} `json:"person"`
	}
	if err := h.rpc(ctx, "GetPerson", map[string]any{"id": h.personId}, &fetched); err != nil {
		return "", err
	}
	if fetched.Person.Id != h.personId || fetched.Person.Name != added.Person.Name {
		return "", fmt.Errorf("read back %+v, want person %d named %q", fetched.Person, h.personId, added.Person.Name)
	}
	return fmt.Sprintf("person %d created and read back", h.personId), nil
}

func (h *harness) stepAddGrowth(ctx context.Context) (string, error) {
	var added struct {
		GrowthData struct {
			Id    int     `json:"id"`
			Value float64 `json:"value"`
		} `json:"growthData"`
	}
	err := h.rpc(ctx, "AddGrowthData", map[string]any{
		"personId":        h.personId,
		"measurementType": "height",
		"value":           104.5,
		"unit":            "cm",
		"inputType":       "today",
		"measurementDate": nil,
		"ageYears":        nil,
		"ageMonths":       nil,
	}, &added)
	if err != nil {
		return "", err
	}
	if added.GrowthData.Id == 0 {
		return "", errors.New("AddGrowthData returned no record")
	}

	var listed struct {
		GrowthData []struct {
			Id int `json:"id"`
		} `json:"growthData"`
	}
	if err := h.rpc(ctx, "GetPerson", map[string]any{"id": h.personId}, &listed); err != nil {
		return "", err
	}
	for _, record := range listed.GrowthData {
		if record.Id == added.GrowthData.Id {
			return fmt.Sprintf("measurement %d on person %d", record.Id, h.personId), nil
		}
	}
	return "", fmt.Errorf("measurement %d is not on person %d's record", added.GrowthData.Id, h.personId)
}

func (h *harness) stepUploadPhoto(ctx context.Context) (string, error) {
	body, contentType, err := photoUploadBody(h.personId)
	if err != nil {
		return "", err
	}

	resp, respBody, err := h.post(ctx, "/api/upload-photo", contentType, body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload: status %d: %s", resp.StatusCode, excerpt(respBody))
	}
	var uploaded struct {
		Image struct {
			Id     int `json:"id"`
			Status int `json:"status"`
		} `json:"image"`
	}
	if err := json.Unmarshal(respBody, &uploaded); err != nil {
		return "", fmt.Errorf("upload returned %s", excerpt(respBody))
	}
	if uploaded.Image.Id == 0 {
		return "", errors.New("upload returned no photo")
	}
	h.photoId = uploaded.Image.Id

	// Face analysis is enabled in a release build and its daemon is not
	// running here. Waiting for the photo to finish processing is therefore
	// also the check that an unavailable subsystem does not take a user's
	// upload down with it.
	if err := h.waitForPhoto(ctx); err != nil {
		return "", err
	}

	header := http.Header{"Accept": {browserAccept}}
	total := 0
	for _, path := range []string{
		fmt.Sprintf("/api/photo/%d/thumb", h.photoId),
		fmt.Sprintf("/api/photo/%d", h.photoId),
	} {
		imgResp, imgBody, err := h.get(ctx, path, header)
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
	return fmt.Sprintf("photo %d processed, thumb + full served (%s)", h.photoId, byteCount(total)), nil
}

// waitForPhoto polls until the background worker has finished the upload. The
// upload handler answers before the work starts, so without this the next
// request would race the worker.
func (h *harness) waitForPhoto(ctx context.Context) error {
	deadline := time.Now().Add(photoTimeout)
	for {
		var status struct {
			Status int `json:"status"`
		}
		if err := h.rpc(ctx, "GetPhotoStatus", map[string]any{"id": h.photoId}, &status); err != nil {
			return err
		}
		switch status.Status {
		case 0:
			return nil
		case 2:
			return fmt.Errorf("photo %d failed processing", h.photoId)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("photo %d still processing after %s", h.photoId, photoTimeout)
		}
		if err := sleep(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
}

func (h *harness) stepChat(ctx context.Context) (string, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	// Origin is set because the server checks it and a browser always sends
	// it; dialing without one would skip a check that a real client faces.
	conn, resp, err := websocket.Dial(dialCtx, h.resolve("/ws/chat"), &websocket.DialOptions{
		HTTPClient: h.http,
		HTTPHeader: http.Header{"Origin": {h.base.String()}},
	})
	if err != nil {
		if resp != nil {
			return "", fmt.Errorf("handshake failed with status %d: %w", resp.StatusCode, err)
		}
		return "", err
	}
	defer conn.Close(websocket.StatusNormalClosure, "e2e complete")

	content := fmt.Sprintf("e2e message %d", time.Now().UnixNano())
	var sent struct {
		Message struct {
			Id      int    `json:"id"`
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := h.rpc(ctx, "SendMessage", map[string]any{"content": content, "clientMessageId": "e2e-1"}, &sent); err != nil {
		return "", err
	}
	if sent.Message.Id == 0 {
		return "", errors.New("SendMessage stored nothing")
	}

	// The message was posted over RPC and has to arrive on the socket: that
	// round trip is the whole point of the chat hub, and it is the half that
	// a handshake-only check cannot see.
	if err := awaitBroadcast(dialCtx, conn, sent.Message.Id); err != nil {
		return "", err
	}

	var history struct {
		Messages []struct {
			Id int `json:"id"`
		} `json:"messages"`
	}
	if err := h.rpc(ctx, "GetChatMessages", map[string]any{}, &history); err != nil {
		return "", err
	}
	for _, message := range history.Messages {
		if message.Id == sent.Message.Id {
			return fmt.Sprintf("message %d broadcast and stored", sent.Message.Id), nil
		}
	}
	return "", fmt.Errorf("message %d is not in the chat history", sent.Message.Id)
}

// awaitBroadcast reads past the presence and typing traffic the hub sends to a
// new connection, looking for the message that was just posted.
func awaitBroadcast(ctx context.Context, conn *websocket.Conn, messageId int) error {
	for {
		var msg struct {
			Type    string `json:"type"`
			Payload struct {
				Message struct {
					Id int `json:"id"`
				} `json:"message"`
			} `json:"payload"`
		}
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return fmt.Errorf("waiting for the message broadcast: %w", err)
		}
		if msg.Type == "new_message" && msg.Payload.Message.Id == messageId {
			return nil
		}
	}
}

func (h *harness) stepLogout(ctx context.Context) (string, error) {
	resp, body, err := h.post(ctx, "/api/logout", "application/json", nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, excerpt(body))
	}
	if h.hasCookie("authToken") {
		return "", errors.New("logout left the authToken cookie in place")
	}

	// A cleared cookie is the browser's half. This is the server's: the same
	// call that worked a moment ago has to be refused now.
	var ignored json.RawMessage
	if err := h.rpc(ctx, "GetPerson", map[string]any{"id": h.personId}, &ignored); err == nil {
		return "", errors.New("GetPerson still answers after logout")
	}
	return "session cleared and refused", nil
}

// ── the deployment under test ──────────────────────────────────────────────

func (h *harness) start(ctx context.Context) error {
	if err := h.preflight(); err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "family-e2e-*")
	if err != nil {
		return err
	}
	h.dir = dir

	// The proxy comes up first because its address is the origin the server is
	// configured with, and the server checks that at startup.
	h.addr = fmt.Sprintf("127.0.0.1:%d", appPort)
	target, err := url.Parse("http://" + h.addr)
	if err != nil {
		return err
	}
	// SingleHostReverseProxy leaves the inbound Host header alone, which is
	// what Caddy does and what the WebSocket origin check depends on.
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorLog = log.New(h.log, "proxy: ", log.LstdFlags)
	h.front = httptest.NewTLSServer(proxy)

	base, err := url.Parse(h.front.URL)
	if err != nil {
		return err
	}
	h.base = base

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	h.http = h.front.Client()
	h.http.Jar = jar
	h.http.Timeout = 60 * time.Second

	env, err := h.serverEnv()
	if err != nil {
		return err
	}
	binary, err := filepath.Abs(h.binary)
	if err != nil {
		return err
	}

	h.cmd = exec.Command(binary)
	h.cmd.Dir = h.dir
	h.cmd.Env = env
	h.cmd.Stdout = h.log
	h.cmd.Stderr = h.log
	if err := h.cmd.Start(); err != nil {
		return fmt.Errorf("starting %s: %w", binary, err)
	}
	h.exit = make(chan error, 1)
	go func() { h.exit <- h.cmd.Wait() }()

	fmt.Printf("e2e: %s serving %s from %s\n", filepath.Base(binary), h.base, h.dir)
	return h.waitForListener(ctx)
}

// preflight refuses to run anywhere that looks like a real deployment. A
// release build's storage paths are compile-time constants, so the scratch
// tree has to be the production one — which makes proving it is unused the
// only thing standing between a test run and someone's photos.
func (h *harness) preflight() error {
	info, err := os.Stat(h.binary)
	if err != nil {
		return fmt.Errorf("no release binary at %s (run `make build` first): %w", h.binary, err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s is not an executable", h.binary)
	}

	// MakeApplication loads its environment from the directory holding the
	// database and static trees (app.go). Deriving that path rather than
	// repeating the literal keeps this in step with cfg if the tree moves.
	shared := filepath.Dir(filepath.Dir(cfg.DBPath))
	if _, err := os.Stat(filepath.Join(shared, ".env")); err == nil {
		return fmt.Errorf("%s exists, so this is a configured deployment; e2e will not run here", filepath.Join(shared, ".env"))
	}
	if _, err := os.Stat(cfg.DBPath); err == nil {
		return fmt.Errorf("a database already exists at %s; e2e will not run against it", cfg.DBPath)
	}

	for _, dir := range []string{filepath.Dir(cfg.DBPath), cfg.StaticDir, cfg.LogDir} {
		if err := ensureScratchDir(dir); err != nil {
			return err
		}
	}

	// An empty static directory is what lets the cleanup remove exactly what
	// the run created, without guessing at which files were already there.
	entries, err := os.ReadDir(cfg.StaticDir)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty; e2e needs a scratch tree, not a populated one", cfg.StaticDir)
	}

	h.owned = true
	return nil
}

func ensureScratchDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return scratchTreeError(dir, err)
	}
	probe, err := os.CreateTemp(dir, ".e2e-*")
	if err != nil {
		return scratchTreeError(dir, err)
	}
	name := probe.Name()
	_ = probe.Close()
	return os.Remove(name)
}

func scratchTreeError(dir string, cause error) error {
	shared := filepath.Dir(filepath.Dir(cfg.DBPath))
	return fmt.Errorf("%s is not usable (%v).\n"+
		"A release build resolves its storage paths at compile time, so e2e needs the tree production would use:\n"+
		"  sudo mkdir -p %s %s\n"+
		"  sudo chown -R \"$(id -un)\" %s",
		dir, cause, filepath.Dir(cfg.DBPath), filepath.Clean(cfg.StaticDir), shared)
}

// serverEnv builds the child's environment explicitly rather than inheriting
// the caller's. A developer's shell holds real API keys and a real database
// path; a run that picked those up would reach services this is not entitled
// to touch and would behave differently on every machine.
func (h *harness) serverEnv() ([]string, error) {
	secret, err := randomSecret()
	if err != nil {
		return nil, err
	}
	backupToken, err := randomSecret()
	if err != nil {
		return nil, err
	}

	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + h.dir,
		"TZ=UTC",
		"JWT_SECRET_KEY=" + secret,
		"BACKUP_TOKEN=" + backupToken,
		"SITE_ROOT=" + h.base.String(),
		// The release config check refuses to serve without these. None of
		// the flows below call Google, Gemini, or a mail relay, so
		// placeholders both satisfy the check and keep the run offline —
		// which is the point of asserting they are only placeholders.
		"GOOGLE_CLIENT_ID=e2e-google-client-id",
		"GOOGLE_CLIENT_SECRET=e2e-google-client-secret",
		"MAIL_FROM=e2e@family-portal.invalid",
	}, nil
}

// randomSecret returns a value long enough for the minimum lengths the release
// build enforces on its secrets, and different on every run.
func randomSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

// waitForListener waits for the process to accept connections, which it only
// does once the config check has passed and every migration has run. Whether it
// is ready to serve is a separate question, asked as the first check of the run
// so that a failure reads as a failed check rather than as startup noise.
func (h *harness) waitForListener(ctx context.Context) error {
	deadline := time.Now().Add(startupTimeout)
	for {
		if err := h.serverAlive(); err != nil {
			return err
		}

		conn, err := net.DialTimeout("tcp", h.addr, time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("server was not listening on %s after %s", h.addr, startupTimeout)
		}
		if err := sleep(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}
}

// serverAlive reports a server that died mid-run as the failure it is, rather
// than letting the next request fail with a connection error that says nothing
// about why. A release build sends everything after the first few lines to a
// rotating log file, so the exit status is often all stdout has to offer.
func (h *harness) serverAlive() error {
	if !h.reap() {
		return nil
	}
	if h.exitErr != nil {
		return fmt.Errorf("server exited early: %w", h.exitErr)
	}
	return errors.New("server exited early, with status 0")
}

// reap collects the process result if it is ready, and reports whether the
// server has exited. It never blocks, so it is safe to call between steps.
func (h *harness) reap() bool {
	if h.exited {
		return true
	}
	if h.exit == nil {
		return false
	}
	select {
	case err := <-h.exit:
		h.exited, h.exitErr = true, err
		return true
	default:
		return false
	}
}

// stop asks the server to drain the way a deploy does and insists it exits
// cleanly. A binary that has to be killed here would be killed by systemd
// there, mid-request.
func (h *harness) stop() error {
	if h.front != nil {
		defer h.front.Close()
	}
	if h.cmd == nil || h.cmd.Process == nil {
		return nil
	}
	if h.reap() {
		return fmt.Errorf("server had already exited: %v", h.exitErr)
	}

	if err := h.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signalling the server: %w", err)
	}

	select {
	case err := <-h.exit:
		h.exited, h.exitErr = true, err
		if err != nil {
			return fmt.Errorf("server exited badly after SIGTERM: %w", err)
		}
		return nil
	case <-time.After(shutdownTimeout):
		_ = h.cmd.Process.Kill()
		h.exited, h.exitErr = true, <-h.exit
		return fmt.Errorf("server did not exit within %s of SIGTERM", shutdownTimeout)
	}
}

func (h *harness) cleanup() {
	if h.keep {
		if h.dir != "" {
			fmt.Printf("e2e: keeping %s and the scratch deployment under %s\n", h.dir, filepath.Dir(filepath.Dir(cfg.DBPath)))
		}
		return
	}
	if h.dir != "" {
		_ = os.RemoveAll(h.dir)
	}
	if !h.owned {
		return
	}
	_ = os.Remove(cfg.DBPath)
	removeContents(cfg.StaticDir)
	removeContents(cfg.LogDir)
}

// removeContents empties a directory without removing the directory itself,
// which may have been created with permissions this process cannot restore.
func removeContents(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
}

// dumpServerLog prints what the server said. A release build redirects the
// standard logger to a rotating file as its second act (app.go), so almost
// everything worth reading is in that file rather than on the pipe. The file is
// under cfg.LogDir, not the child's working directory — logs live in shared/ so
// they survive a deploy.
func (h *harness) dumpServerLog() {
	sections := []struct {
		name string
		text string
	}{
		{"output", h.log.tail()},
		{"log file", tailFile(filepath.Join(cfg.LogDir, "family_record.log"))},
	}
	for _, section := range sections {
		if section.text == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "\n─── server %s, last %d lines ───\n%s\n───\n", section.name, maxLogLines, section.text)
	}
}

func tailFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) > maxLogLines {
		lines = lines[len(lines)-maxLogLines:]
	}
	return strings.Join(lines, "\n")
}

// ── HTTP plumbing ──────────────────────────────────────────────────────────

// rpc calls a vbeam procedure the way the generated frontend client does and
// decodes its result. vbeam answers a failed procedure with 400 and the error
// text as the body, so a non-200 here is the procedure's own message.
func (h *harness) rpc(ctx context.Context, name string, request any, response any) error {
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	resp, body, err := h.post(ctx, "/rpc/"+name, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d: %s", name, resp.StatusCode, excerpt(body))
	}
	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("%s returned %s", name, excerpt(body))
	}
	return nil
}

func (h *harness) get(ctx context.Context, path string, header http.Header) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.resolve(path), nil)
	if err != nil {
		return nil, nil, err
	}
	for name, values := range header {
		req.Header[name] = values
	}
	return h.do(req)
}

func (h *harness) post(ctx context.Context, path, contentType string, body io.Reader) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.resolve(path), body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return h.do(req)
}

func (h *harness) do(req *http.Request) (*http.Response, []byte, error) {
	resp, err := h.http.Do(req)
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

func (h *harness) resolve(path string) string {
	ref, err := url.Parse(path)
	if err != nil {
		return h.base.String() + path
	}
	return h.base.ResolveReference(ref).String()
}

func (h *harness) hasCookie(name string) bool {
	for _, cookie := range h.http.Jar.Cookies(h.base) {
		if cookie.Name == name && cookie.Value != "" {
			return true
		}
	}
	return false
}

// photoUploadBody builds the multipart body the upload page sends. The part's
// Content-Type is set explicitly because the handler validates the uploaded
// type from it, and multipart's own helper would label the file as
// application/octet-stream.
func photoUploadBody(personId int) (io.Reader, string, error) {
	img := image.NewRGBA(image.Rect(0, 0, 96, 96))
	for y := 0; y < 96; y++ {
		for x := 0; x < 96; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 128, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, nil); err != nil {
		return nil, "", err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"title":     "E2E upload",
		"inputType": "today",
		"personIds": fmt.Sprintf("[%d]", personId),
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}

	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="photo"; filename="e2e.jpg"`)
	header.Set("Content-Type", "image/jpeg")
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(encoded.Bytes()); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

// ── odds and ends ──────────────────────────────────────────────────────────

// serverLog keeps the tail of the server's own output, so a failed step can
// show what the process said without a passing run printing a startup log.
type serverLog struct {
	mu      sync.Mutex
	echo    bool
	pending []byte
	lines   []string
}

func (l *serverLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.echo {
		_, _ = os.Stderr.Write(p)
	}

	l.pending = append(l.pending, p...)
	for {
		end := bytes.IndexByte(l.pending, '\n')
		if end < 0 {
			break
		}
		l.lines = append(l.lines, string(l.pending[:end]))
		l.pending = l.pending[end+1:]
	}
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
	}
	return len(p), nil
}

func (l *serverLog) tail() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	lines := l.lines
	if len(l.pending) > 0 {
		lines = append(append([]string{}, lines...), string(l.pending))
	}
	return strings.Join(lines, "\n")
}

func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// excerpt renders a response body short enough to sit on one line. Bodies here
// come from a server that is already misbehaving, so they may be anything.
func excerpt(body []byte) string {
	text := strings.TrimSpace(string(body))
	if idx := strings.IndexAny(text, "\r\n"); idx >= 0 {
		text = text[:idx]
	}
	if len(text) > 160 {
		text = text[:160] + "..."
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
