package backend

import (
	"encoding/json"
	"net/http"
	"strings"

	"go.hasen.dev/vbeam"
)

const ClientErrorPath = "/api/client-error"

const (
	maxClientErrorField = 1000
	maxClientErrorStack = 4000
)

type ClientErrorReport struct {
	Message string `json:"message"`
	Stack   string `json:"stack"`
	Route   string `json:"route"`
	Source  string `json:"source"`
}

func RegisterClientErrorHandlers(app *vbeam.Application) {
	app.HandleFunc("POST "+ClientErrorPath, clientErrorHandler)
}

// Unauthenticated: the errors most worth seeing are the ones that break the page
// before anyone can sign in. The rate limiter and the field caps are what keep
// it from becoming a place to write arbitrary volume into the log.
func clientErrorHandler(w http.ResponseWriter, r *http.Request) {
	var report ClientErrorReport
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClientErrorStack*2)).Decode(&report); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	report.Message = truncate(strings.TrimSpace(report.Message), maxClientErrorField)
	report.Route = truncate(strings.TrimSpace(report.Route), maxClientErrorField)
	report.Source = truncate(strings.TrimSpace(report.Source), maxClientErrorField)
	report.Stack = truncate(strings.TrimSpace(report.Stack), maxClientErrorStack)

	if report.Message == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	data := map[string]interface{}{
		"clientMessage": report.Message,
		"route":         report.Route,
		"source":        report.Source,
		"userAgent":     truncate(r.UserAgent(), maxClientErrorField),
	}
	if report.Stack != "" {
		data["stack"] = report.Stack
	}
	if user, err := AuthenticateRequest(r); err == nil {
		data["userId"] = user.Id
	}

	// Warn, not error: a broken browser extension or one user's flaky network
	// should show up in the log without flipping the health check red and
	// mailing the admin.
	LogWarnWithRequest(r, LogCategoryClient, "Client-side error reported", data)

	w.WriteHeader(http.StatusNoContent)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
