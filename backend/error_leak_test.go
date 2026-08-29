package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

var forbiddenInResponses = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"absolute filesystem path", regexp.MustCompile(`(^|[^a-zA-Z0-9])/(srv|home|var|usr|tmp|etc)/`)},
	{"Go source location", regexp.MustCompile(`\.go:\d+`)},
	{"stack frame", regexp.MustCompile(`goroutine \d+|runtime\.|panic:`)},
	{"bolt or vbolt internals", regexp.MustCompile(`(?i)vbolt|boltdb|bucket|transaction`)},
	{"environment variable name", regexp.MustCompile(`[A-Z][A-Z0-9_]{4,}_(KEY|SECRET|TOKEN|PASSWORD)`)},
	{"bearer or JWT material", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}`)},
}

func assertNoLeak(t *testing.T, body string) {
	t.Helper()

	for _, forbidden := range forbiddenInResponses {
		if forbidden.pattern.MatchString(body) {
			t.Errorf("response exposes a %s:\n%s", forbidden.name, body)
		}
	}
}

func TestUnexpectedErrorNeverEchoesItsCause(t *testing.T) {
	causes := []error{
		&os.PathError{Op: "open", Path: "/srv/apps/family/shared/data/db.bolt", Err: os.ErrPermission},
		errorString("vbolt: bucket \"users\" write transaction failed at bolt.go:117"),
		errorString("JWT_SECRET_KEY is only 4 characters"),
		errorString("token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc rejected"),
	}

	for _, cause := range causes {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/anything", nil)

		RespondUnexpectedError(recorder, req, cause)

		body := recorder.Body.String()
		assertNoLeak(t, body)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}

		var response ErrorResponse
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			t.Fatalf("response is not JSON: %v", err)
		}
		if response.Error.Message != unexpectedErrorMessage {
			t.Errorf("message = %q, want the fixed message %q", response.Error.Message, unexpectedErrorMessage)
		}
	}
}

func TestProcErrorNeverEchoesItsCause(t *testing.T) {
	cause := errorString("open /srv/apps/family/shared/static/photos/1.jpg: permission denied")

	safe := ProcError(cause)
	if safe == nil {
		t.Fatal("ProcError dropped the error entirely")
	}
	assertNoLeak(t, safe.Error())

	if !strings.HasPrefix(safe.Error(), unexpectedErrorMessage) {
		t.Errorf("message = %q, want it to start with the fixed message", safe.Error())
	}
}

func TestProcErrorCarriesAReference(t *testing.T) {
	safe := ProcError(errorString("something internal"))

	reference := regexp.MustCompile(regexp.QuoteMeta(ReferencePrefix) + `([0-9a-f]{12})`)
	if !reference.MatchString(safe.Error()) {
		t.Errorf("no reference code in %q", safe.Error())
	}
}

func TestProcErrorPassesDeclaredErrorsThrough(t *testing.T) {
	for _, declared := range publicErrors {
		if got := ProcError(declared); got != declared {
			t.Errorf("ProcError(%v) = %v, want it returned unchanged", declared, got)
		}
	}
}

func TestProcErrorOnNilIsNil(t *testing.T) {
	if got := ProcError(nil); got != nil {
		t.Errorf("ProcError(nil) = %v, want nil", got)
	}
}

func TestNoResponderLeaksItsDetails(t *testing.T) {
	const secret = "open /srv/apps/family/shared/.env: permission denied"

	responders := map[string]func(http.ResponseWriter, *http.Request){
		"validation": func(w http.ResponseWriter, r *http.Request) {
			RespondValidationError(w, r, "That did not look right.", secret)
		},
		"internal": func(w http.ResponseWriter, r *http.Request) {
			RespondInternalError(w, r, "Something went wrong on our end.", secret)
		},
		"unavailable": func(w http.ResponseWriter, r *http.Request) {
			RespondUnavailableError(w, r, "Try again shortly.", secret)
		},
	}

	for name, respond := range responders {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			respond(recorder, httptest.NewRequest(http.MethodGet, "/api/thing", nil))

			if strings.Contains(recorder.Body.String(), secret) {
				t.Errorf("details reached the body: %s", recorder.Body.String())
			}
			assertNoLeak(t, recorder.Body.String())
		})
	}
}

func TestEveryResponseCarriesACorrelationId(t *testing.T) {
	var seen string
	wrapper := NewRequestIDWrapper(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r)
		w.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	wrapper.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/anything", nil))

	header := recorder.Header().Get(RequestIDHeader)
	if header == "" {
		t.Fatal("no correlation id on the response")
	}
	if header != seen {
		t.Errorf("handler saw %q, response carried %q", seen, header)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(header) {
		t.Errorf("correlation id %q is not twelve hex characters", header)
	}
}

func TestCorrelationIdsDiffer(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewRequestID()
		if seen[id] {
			t.Fatalf("NewRequestID repeated %q within 100 calls", id)
		}
		seen[id] = true
	}
}

func TestRequestIDIsEmptyWithoutTheWrapper(t *testing.T) {
	if got := RequestID(httptest.NewRequest(http.MethodGet, "/", nil)); got != "" {
		t.Errorf("RequestID = %q, want empty", got)
	}
	if got := RequestID(nil); got != "" {
		t.Errorf("RequestID(nil) = %q, want empty", got)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }
